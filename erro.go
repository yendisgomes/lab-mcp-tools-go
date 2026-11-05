package main

import (
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Erro é um erro de negócio com código estável (para lógica programática)
// e sugestão de correção (para o agente se ajustar sozinho).
type Erro struct {
	Codigo   string `json:"erro"`
	Mensagem string `json:"mensagem"`
	Sugestao string `json:"sugestao,omitempty"`
}

func (e *Erro) Error() string { return e.Mensagem }

// resultadoErro devolve o erro como resultado da tool (isError: true), com o
// corpo em JSON estável: {"erro": código, "mensagem": ..., "sugestao": ...}.
func resultadoErro(e *Erro) *mcp.CallToolResult {
	corpo, _ := json.Marshal(e)
	res := &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(corpo)}}}
	res.SetError(e)
	return res
}
