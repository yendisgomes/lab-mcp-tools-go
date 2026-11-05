package main

import (
	"encoding/json"

	"github.com/google/jsonschema-go/jsonschema"
)

func ptr[T any](v T) *T { return &v }

func enum(valores ...string) []any {
	out := make([]any, len(valores))
	for i, v := range valores {
		out[i] = v
	}
	return out
}

type schemasFrete struct {
	cotarEntrada, cotarSaida, contratarEntrada, contratarSaida *jsonschema.Schema
}

// criarSchemasFrete parte dos schemas inferidos dos tipos e acrescenta o que as
// tags não expressam: padrões, limites, enums e o default de dry_run.
func criarSchemasFrete() (*schemasFrete, error) {
	var s schemasFrete
	var err error
	if s.cotarEntrada, err = jsonschema.For[EntradaCotacao](nil); err != nil {
		return nil, err
	}
	if s.cotarSaida, err = jsonschema.For[SaidaCotacao](nil); err != nil {
		return nil, err
	}
	if s.contratarEntrada, err = jsonschema.For[EntradaContratacao](nil); err != nil {
		return nil, err
	}
	if s.contratarSaida, err = jsonschema.For[SaidaContratacao](nil); err != nil {
		return nil, err
	}

	c := s.cotarEntrada.Properties
	for _, campo := range []string{"origem_cep", "destino_cep"} {
		c[campo].Pattern = `^\d{5}-?\d{3}$`
		c[campo].Examples = []any{"01310-100", "80010000"}
	}
	c["itens"].MinItems, c["itens"].MaxItems = ptr(1), ptr(20)
	item := c["itens"].Items.Properties
	item["sku"].MinLength = ptr(1)
	item["quantidade"].Minimum, item["quantidade"].Maximum = ptr(1.0), ptr(100.0)
	item["peso_kg"].ExclusiveMinimum = ptr(0.0)
	item["valor_unitario"].Minimum = ptr(0.0)
	item["categoria"].Enum = enum("comum", "perecivel", "liquido", "bateria", "inflamavel")
	item["categoria"].Default = json.RawMessage(`"comum"`)
	for _, campo := range []string{"comprimento", "largura", "altura"} {
		item["dimensoes"].Properties[campo].ExclusiveMinimum = ptr(0.0)
	}
	c["servicos"].MinItems, c["servicos"].UniqueItems = ptr(1), true
	c["servicos"].Items.Enum = enum("economico", "padrao", "expresso")
	c["prazo_maximo_dias"].Minimum, c["prazo_maximo_dias"].Maximum = ptr(1.0), ptr(30.0)

	k := s.contratarEntrada.Properties
	k["cotacao_id"].Pattern = `^COT-\d{6}$`
	k["servico"].Enum = enum("economico", "padrao", "expresso")
	k["chave_idempotencia"].MinLength, k["chave_idempotencia"].MaxLength = ptr(8), ptr(64)
	k["dry_run"].Default = json.RawMessage(`true`)
	k["destinatario"].Properties["nome"].MinLength = ptr(1)
	k["endereco"].Properties["uf"].Pattern = `^[A-Za-z]{2}$`
	return &s, nil
}
