package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// version é preenchida no build: -ldflags "-X main.version=1.2.3".
var version = "dev"

const instrucoes = "Cote com cotar_frete antes de contratar. contratar_envio é dry_run por padrão; " +
	"só envie dry_run=false depois de o usuário confirmar o resumo, reutilizando a mesma chave_idempotencia."

func novoServidor() (*mcp.Server, error) {
	server := mcp.NewServer(
		&mcp.Implementation{Name: "mcp-frete", Version: version},
		&mcp.ServerOptions{Instructions: instrucoes},
	)
	if err := registrarFrete(server); err != nil {
		return nil, err
	}
	return server, nil
}

// novoHandlerHTTP serve o mesmo servidor via Streamable HTTP em /mcp, para
// ficar atrás de um gateway (Kong). /healthz serve probes de liveness.
func novoHandlerHTTP(server *mcp.Server) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	return mux
}

// servirHTTP bloqueia até ctx ser cancelado e então encerra com graça.
func servirHTTP(ctx context.Context, addr string, h http.Handler) error {
	srv := &http.Server{Addr: addr, Handler: h, ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func main() {
	addr := flag.String("http", "", "endereço HTTP (ex.: :8080); vazio usa stdio")
	flag.Parse()
	log.SetPrefix("[mcp-frete] ") // logs vão para stderr; stdout é o protocolo

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server, err := novoServidor()
	if err != nil {
		log.Fatal(err)
	}
	if *addr != "" {
		log.Printf("Streamable HTTP em %s/mcp (versão %s)", *addr, version)
		if err := servirHTTP(ctx, *addr, novoHandlerHTTP(server)); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
		return
	}
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}
