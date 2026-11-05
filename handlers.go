package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (a *armazem) cotarFrete(ctx context.Context, req *mcp.CallToolRequest, in EntradaCotacao) (*mcp.CallToolResult, any, error) {
	log.Printf("cotar_frete: %d linhas %s -> %s servicos=%v seguro=%v", len(in.Itens), in.OrigemCEP, in.DestinoCEP, in.Servicos, in.Seguro)
	out, e := calcularCotacao(in)
	if e != nil {
		return resultadoErro(e), nil, nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.seq++
	out.CotacaoID = fmt.Sprintf("COT-%06d", a.seq)
	valida := a.agora().Add(validadeCotacao)
	out.ValidaAte = valida.Format(time.RFC3339)
	a.cotacoes[out.CotacaoID] = cotacaoSalva{out, valida}
	return nil, out, nil
}

func validarDestino(d Destinatario, e Endereco) *Erro {
	if n := len(digitos(d.Documento)); n != 11 && n != 14 {
		return &Erro{erroDocumentoInvalido, "documento deve ter 11 (CPF) ou 14 (CNPJ) dígitos", "Peça o documento correto ao usuário; não invente."}
	}
	if d.Email != "" && !strings.Contains(d.Email, "@") {
		return &Erro{erroEmailInvalido, fmt.Sprintf("e-mail %q inválido", d.Email), "Corrija o e-mail ou omita o campo."}
	}
	if !ufs[strings.ToUpper(e.UF)] {
		return &Erro{erroUFInvalida, fmt.Sprintf("UF %q inválida", e.UF), "Use a sigla de 2 letras de um estado brasileiro."}
	}
	return nil
}

func impressao(in EntradaContratacao) string {
	corpo, _ := json.Marshal(struct {
		C string
		S string
		D Destinatario
		E Endereco
	}{in.CotacaoID, in.Servico, in.Destinatario, in.Endereco})
	h := sha256.Sum256(corpo)
	return hex.EncodeToString(h[:])
}

func (a *armazem) contratarEnvio(ctx context.Context, req *mcp.CallToolRequest, in EntradaContratacao) (*mcp.CallToolResult, any, error) {
	dry := in.DryRun == nil || *in.DryRun
	log.Printf("contratar_envio: cotacao=%s servico=%s dry_run=%v chave=%s", in.CotacaoID, in.Servico, dry, in.ChaveIdempotencia)

	a.mu.Lock()
	defer a.mu.Unlock()

	imp := impressao(in)
	if !dry {
		if s, ok := a.envios[in.ChaveIdempotencia]; ok {
			if s.impressao != imp {
				return resultadoErro(&Erro{erroConflitoIdempotencia,
					fmt.Sprintf("a chave %q já foi usada para um pedido diferente", in.ChaveIdempotencia),
					"Use uma chave nova para um envio novo; reutilize a mesma só para repetir o mesmo pedido."}), nil, nil
			}
			out := s.saida
			out.Reaproveitado = true
			return nil, out, nil
		}
	}

	cot, ok := a.cotacoes[in.CotacaoID]
	if !ok {
		return resultadoErro(&Erro{erroCotacaoNaoEncontrada, fmt.Sprintf("cotação %q não existe", in.CotacaoID), "Chame cotar_frete e use o cotacao_id retornado."}), nil, nil
	}
	if a.agora().After(cot.valida) {
		return resultadoErro(&Erro{erroCotacaoExpirada, fmt.Sprintf("a cotação %s expirou em %s", in.CotacaoID, cot.valida.Format(time.RFC3339)), "Cote de novo e confirme o preço com o usuário antes de contratar."}), nil, nil
	}
	if outra, usada := a.usadas[in.CotacaoID]; usada {
		return resultadoErro(&Erro{erroCotacaoUtilizada, fmt.Sprintf("a cotação %s já gerou o envio da chave %q", in.CotacaoID, outra), "Cada cotação gera um único envio; cote de novo para outro envio."}), nil, nil
	}
	var op *Opcao
	var disponiveis []string
	for i := range cot.saida.Opcoes {
		disponiveis = append(disponiveis, cot.saida.Opcoes[i].Servico)
		if cot.saida.Opcoes[i].Servico == in.Servico {
			op = &cot.saida.Opcoes[i]
		}
	}
	if op == nil {
		return resultadoErro(&Erro{erroServicoIndisponivel, fmt.Sprintf("o serviço %q não está nas opções da cotação", in.Servico), fmt.Sprintf("Escolha entre: %s.", strings.Join(disponiveis, ", "))}), nil, nil
	}
	if e := validarDestino(in.Destinatario, in.Endereco); e != nil {
		return resultadoErro(e), nil, nil
	}

	previsao := a.agora().AddDate(0, 0, op.PrazoDias).Format("2006-01-02")
	out := SaidaContratacao{Status: "simulado", Servico: op.Servico, PrecoTotal: op.Preco, PrevisaoEntrega: previsao}
	if dry {
		out.Texto = fmt.Sprintf("Simulação: %s por R$ %.2f, entrega prevista em %s (nada foi contratado)", op.Servico, op.Preco, previsao)
		return nil, out, nil
	}

	h := sha256.Sum256([]byte(in.ChaveIdempotencia))
	out.Status = "contratado"
	out.EnvioID = "ENV-" + strings.ToUpper(hex.EncodeToString(h[:4]))
	out.CodigoRastreio = fmt.Sprintf("LAB%09dBR", (uint64(h[4])<<24|uint64(h[5])<<16|uint64(h[6])<<8|uint64(h[7]))%1_000_000_000)
	out.Texto = fmt.Sprintf("Envio %s contratado via %s por R$ %.2f, entrega prevista em %s, rastreio %s", out.EnvioID, op.Servico, op.Preco, previsao, out.CodigoRastreio)
	a.envios[in.ChaveIdempotencia] = envioSalvo{imp, out}
	a.usadas[in.CotacaoID] = in.ChaveIdempotencia
	return nil, out, nil
}
