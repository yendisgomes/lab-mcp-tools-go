package main

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
)

// Regras de negócio puras: não conhecem MCP nem estado.

const (
	validadeCotacao = 15 * time.Minute

	limitePesoEconomicoKg = 30.0
	limitePesoTotalKg     = 70.0
	limiteDimensaoCm      = 150.0
	limiteSeguroAvisoR    = 1000.0
	limiteDeclaradoR      = 10000.0
	taxaSeguro            = 0.012
	seguroMinimoR         = 5.0
	taxaFragilR           = 6.0
	taxaLiquidoR          = 8.0
	divisorCubagem        = 6000.0
)

const (
	erroCEPInvalido          = "cep_invalido"
	erroItemProibido         = "item_proibido"
	erroDimensaoExcedida     = "dimensao_excede_limite"
	erroPesoExcedido         = "peso_excede_limite"
	erroValorDeclarado       = "valor_declarado_excede_limite"
	erroPerecivelForaRegiao  = "perecivel_fora_da_regiao"
	erroCotacaoNaoEncontrada = "cotacao_nao_encontrada"
	erroCotacaoExpirada      = "cotacao_expirada"
	erroCotacaoUtilizada     = "cotacao_ja_utilizada"
	erroServicoIndisponivel  = "servico_indisponivel_na_cotacao"
	erroDocumentoInvalido    = "documento_invalido"
	erroEmailInvalido        = "email_invalido"
	erroUFInvalida           = "uf_invalida"
	erroConflitoIdempotencia = "conflito_idempotencia"
)

type servicoFrete struct {
	nome         string
	base, porKg  float64
	prazoBase    int
	prazoPorZona int
}

var tabelaServicos = []servicoFrete{
	{"economico", 14, 1.8, 5, 2},
	{"padrao", 22, 2.6, 3, 1},
	{"expresso", 38, 4.2, 1, 1},
}

var (
	fatorZona = map[int]float64{1: 1, 2: 1.25, 3: 1.6}
	nomeZona  = map[int]string{1: "local", 2: "regional", 3: "nacional"}
)

var ufs = map[string]bool{
	"AC": true, "AL": true, "AP": true, "AM": true, "BA": true, "CE": true, "DF": true,
	"ES": true, "GO": true, "MA": true, "MT": true, "MS": true, "MG": true, "PA": true,
	"PB": true, "PR": true, "PE": true, "PI": true, "RJ": true, "RN": true, "RS": true,
	"RO": true, "RR": true, "SC": true, "SP": true, "SE": true, "TO": true,
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

func digitos(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
}

// zonaEntre classifica a distância pelo primeiro dígito dos CEPs: mesmo dígito
// é local, até 3 de diferença é regional, o resto é nacional.
func zonaEntre(origem, destino string) int {
	diff := int(origem[0]) - int(destino[0])
	if diff < 0 {
		diff = -diff
	}
	switch {
	case diff == 0:
		return 1
	case diff <= 3:
		return 2
	}
	return 3
}

// calcularCotacao aplica as regras de negócio e devolve a cotação sem id nem
// validade. Os erros são *Erro; os problemas recuperáveis viram descartadas e avisos.
func calcularCotacao(in EntradaCotacao) (SaidaCotacao, *Erro) {
	origem, destino := digitos(in.OrigemCEP), digitos(in.DestinoCEP)
	if len(origem) != 8 || len(destino) != 8 {
		return SaidaCotacao{}, &Erro{erroCEPInvalido, "CEP de origem ou destino não tem 8 dígitos", "Informe CEPs no formato 12345-678."}
	}
	zona := zonaEntre(origem, destino)

	out := SaidaCotacao{Zona: nomeZona[zona], Opcoes: []Opcao{}, Descartadas: []Descarte{}, Avisos: []string{}}
	var (
		pesoTaxado, declarado, taxas float64
		perecivel, bateria           bool
		vistos                       = map[string]bool{}
	)
	for _, it := range in.Itens {
		cat := it.Categoria
		if cat == "" {
			cat = "comum"
		}
		if cat == "inflamavel" {
			return SaidaCotacao{}, &Erro{erroItemProibido,
				fmt.Sprintf("o item %q é inflamável e não pode ser enviado", it.SKU),
				"Remova o item do pacote; não há serviço que o transporte."}
		}
		d := it.Dimensoes
		if maior := max(d.Comprimento, d.Largura, d.Altura); maior > limiteDimensaoCm {
			return SaidaCotacao{}, &Erro{erroDimensaoExcedida,
				fmt.Sprintf("o item %q tem %v cm numa dimensão (limite %v cm)", it.SKU, maior, limiteDimensaoCm),
				"Divida o item em volumes menores ou informe as dimensões corretas."}
		}
		cubado := d.Comprimento * d.Largura * d.Altura / divisorCubagem
		pesoTaxado += max(it.PesoKg, cubado) * float64(it.Quantidade)
		declarado += it.ValorUnitario * float64(it.Quantidade)

		perecivel = perecivel || cat == "perecivel"
		bateria = bateria || cat == "bateria"
		if it.Fragil {
			taxas += taxaFragilR
			out.Avisos = append(out.Avisos, fmt.Sprintf("item %q é frágil: taxa de manuseio de R$ %.2f aplicada", it.SKU, taxaFragilR))
		}
		if cat == "liquido" {
			taxas += taxaLiquidoR
			out.Avisos = append(out.Avisos, fmt.Sprintf("item %q é líquido: embalagem reforçada de R$ %.2f aplicada", it.SKU, taxaLiquidoR))
		}
		if vistos[it.SKU] {
			out.Avisos = append(out.Avisos, fmt.Sprintf("SKU %q aparece em mais de uma linha; considere somar as quantidades", it.SKU))
		}
		vistos[it.SKU] = true
	}
	pesoTaxado, declarado = round2(pesoTaxado), round2(declarado)
	out.PesoTaxadoKg, out.ValorDeclarado = pesoTaxado, declarado

	if pesoTaxado > limitePesoTotalKg {
		return SaidaCotacao{}, &Erro{erroPesoExcedido,
			fmt.Sprintf("peso taxado de %v kg excede o limite de %v kg por pacote", pesoTaxado, limitePesoTotalKg),
			"Divida o pedido em mais de um pacote e cote cada um."}
	}
	if declarado > limiteDeclaradoR {
		return SaidaCotacao{}, &Erro{erroValorDeclarado,
			fmt.Sprintf("valor declarado de R$ %.2f excede R$ %.2f", declarado, limiteDeclaradoR),
			"Divida o pedido em mais de um pacote."}
	}
	if perecivel && zona == 3 {
		return SaidaCotacao{}, &Erro{erroPerecivelForaRegiao,
			"item perecível não pode seguir para uma zona nacional",
			"Use CEPs da mesma região (diferença de até 3 no primeiro dígito) ou remova o item."}
	}
	if declarado > limiteSeguroAvisoR && !in.Seguro {
		out.Avisos = append(out.Avisos, fmt.Sprintf("valor declarado de R$ %.2f sem seguro; considere seguro=true", declarado))
	}

	seguro := 0.0
	if in.Seguro {
		seguro = max(seguroMinimoR, round2(declarado*taxaSeguro))
	}
	pedidos := in.Servicos
	if len(pedidos) == 0 {
		pedidos = []string{"economico", "padrao", "expresso"}
	}
	for _, s := range tabelaServicos {
		if !slices.Contains(pedidos, s.nome) {
			continue
		}
		prazo := s.prazoBase + s.prazoPorZona*(zona-1)
		var motivo string
		switch {
		case perecivel && s.nome != "expresso":
			motivo = "item perecível exige expresso"
		case bateria && s.nome == "economico":
			motivo = "bateria não segue no econômico"
		case pesoTaxado > limitePesoEconomicoKg && s.nome == "economico":
			motivo = fmt.Sprintf("peso acima de %v kg não segue no econômico", limitePesoEconomicoKg)
		case in.PrazoMaximoDias != nil && prazo > *in.PrazoMaximoDias:
			motivo = fmt.Sprintf("prazo de %d dias excede o máximo de %d", prazo, *in.PrazoMaximoDias)
		}
		if motivo != "" {
			out.Descartadas = append(out.Descartadas, Descarte{s.nome, motivo})
			continue
		}
		preco := round2((s.base+s.porKg*pesoTaxado)*fatorZona[zona] + taxas + seguro)
		out.Opcoes = append(out.Opcoes, Opcao{s.nome, preco, prazo, in.Seguro})
	}
	if len(out.Opcoes) == 0 {
		out.Avisos = append(out.Avisos, "nenhuma opção atende às restrições; veja descartadas")
	}
	return out, nil
}
