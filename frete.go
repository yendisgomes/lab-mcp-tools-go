package main

import "github.com/modelcontextprotocol/go-sdk/mcp"

// Frete simula um fluxo de produção em duas tools: cotar_frete calcula opções
// e devolve uma cotação com validade; contratar_envio consome essa cotação,
// com dry_run por padrão e chave de idempotência. O estado fica em memória.

func registrarFrete(server *mcp.Server) error {
	s, err := criarSchemasFrete()
	if err != nil {
		return err
	}
	a := novoArmazem()
	fechado := false
	mcp.AddTool(server, &mcp.Tool{
		Name:  "cotar_frete",
		Title: "Cotar frete",
		Description: "Cota o frete de um pacote (economico, padrao e expresso) e devolve um cotacao_id válido por 15 minutos. " +
			"Aplica restrições por categoria, peso e prazo; o que não é oferecido volta em descartadas, com o motivo.",
		InputSchema:  s.cotarEntrada,
		OutputSchema: s.cotarSaida,
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: ptr(false),
			OpenWorldHint:   &fechado,
		},
	}, a.cotarFrete)
	mcp.AddTool(server, &mcp.Tool{
		Name:  "contratar_envio",
		Title: "Contratar envio",
		Description: "Contrata o envio de uma cotação. Por padrão é dry_run (só valida e simula); com dry_run=false " +
			"contrata de verdade e não dá para desfazer. Exige chave_idempotencia: repetir o mesmo pedido com a mesma chave devolve o mesmo envio.",
		InputSchema:  s.contratarEntrada,
		OutputSchema: s.contratarSaida,
		Annotations: &mcp.ToolAnnotations{
			DestructiveHint: ptr(true),
			IdempotentHint:  true,
			OpenWorldHint:   &fechado,
		},
	}, a.contratarEnvio)
	return nil
}
