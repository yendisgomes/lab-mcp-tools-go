package main

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// conectar sobe o servidor e um cliente MCP ligados por transporte em memória,
// exercitando o protocolo de verdade (schema, validação, isError).
func conectar(t *testing.T) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	server, err := novoServidor()
	if err != nil {
		t.Fatal(err)
	}
	ct, st := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ss.Close() })
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "teste", Version: "0"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func TestStreamableHTTP(t *testing.T) {
	server, err := novoServidor()
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(novoHandlerHTTP(server))
	t.Cleanup(srv.Close)

	cs, err := mcp.NewClient(&mcp.Implementation{Name: "teste", Version: "0"}, nil).
		Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: srv.URL + "/mcp"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })

	cot := decodificar[SaidaCotacao](t, chamarTool(t, cs, "cotar_frete", argsCotacao()))
	if cot.CotacaoID == "" || len(cot.Opcoes) != 3 {
		t.Fatalf("cotação = %+v", cot)
	}
	sim := decodificar[SaidaContratacao](t, chamarTool(t, cs, "contratar_envio", argsContratacao(cot.CotacaoID, "pedido-http-1", nil)))
	if sim.Status != "simulado" {
		t.Fatalf("simulação = %+v", sim)
	}
}
