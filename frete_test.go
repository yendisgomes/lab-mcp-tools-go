package main

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// itemBase pesa 1 kg e cabe numa caixa de 10 cm, então o peso cubado (0,17 kg)
// nunca vence o real. Com CEPs locais: economico 15.8, padrao 24.6, expresso 42.2.
func itemBase() Item {
	return Item{SKU: "A1", Quantidade: 1, PesoKg: 1, Dimensoes: Dimensoes{10, 10, 10}, ValorUnitario: 100}
}

func cotacaoLocal(itens ...Item) EntradaCotacao {
	return EntradaCotacao{OrigemCEP: "01310-100", DestinoCEP: "01999-999", Itens: itens}
}

func proximo(a, b float64) bool { return math.Abs(a-b) < 0.005 }

func opcaoDe(t *testing.T, s SaidaCotacao, servico string) Opcao {
	t.Helper()
	for _, o := range s.Opcoes {
		if o.Servico == servico {
			return o
		}
	}
	t.Fatalf("opção %q ausente em %+v", servico, s.Opcoes)
	return Opcao{}
}

func TestCalcularCotacaoPrecosPorZona(t *testing.T) {
	casos := []struct {
		nome    string
		destino string
		zona    string
		padrao  float64
		prazo   int
	}{
		{"local", "01999-999", "local", 24.6, 3},
		{"regional", "30000-000", "regional", 30.75, 4},
		{"nacional", "80000-000", "nacional", 39.36, 5},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			in := cotacaoLocal(itemBase())
			in.DestinoCEP = c.destino
			got, e := calcularCotacao(in)
			if e != nil {
				t.Fatal(e)
			}
			if got.Zona != c.zona {
				t.Errorf("zona = %q, esperado %q", got.Zona, c.zona)
			}
			if o := opcaoDe(t, got, "padrao"); !proximo(o.Preco, c.padrao) || o.PrazoDias != c.prazo {
				t.Errorf("padrao = %+v, esperado preço %v e prazo %d", o, c.padrao, c.prazo)
			}
		})
	}
}

func TestCalcularCotacaoPesoCubadoVenceOReal(t *testing.T) {
	it := itemBase()
	it.PesoKg = 1
	it.Dimensoes = Dimensoes{60, 40, 30} // 72000/6000 = 12 kg
	got, e := calcularCotacao(cotacaoLocal(it))
	if e != nil {
		t.Fatal(e)
	}
	if got.PesoTaxadoKg != 12 {
		t.Errorf("peso taxado = %v, esperado 12", got.PesoTaxadoKg)
	}
}

func TestCalcularCotacaoRestricoesDeCategoria(t *testing.T) {
	t.Run("perecível só expresso", func(t *testing.T) {
		it := itemBase()
		it.Categoria = "perecivel"
		got, e := calcularCotacao(cotacaoLocal(it))
		if e != nil {
			t.Fatal(e)
		}
		if len(got.Opcoes) != 1 || got.Opcoes[0].Servico != "expresso" || len(got.Descartadas) != 2 {
			t.Errorf("opções = %+v, descartadas = %+v", got.Opcoes, got.Descartadas)
		}
	})
	t.Run("perecível fora da região", func(t *testing.T) {
		it := itemBase()
		it.Categoria = "perecivel"
		in := cotacaoLocal(it)
		in.DestinoCEP = "80000-000"
		if _, e := calcularCotacao(in); e == nil || e.Codigo != erroPerecivelForaRegiao {
			t.Errorf("erro = %+v", e)
		}
	})
	t.Run("bateria sem econômico", func(t *testing.T) {
		it := itemBase()
		it.Categoria = "bateria"
		got, _ := calcularCotacao(cotacaoLocal(it))
		if len(got.Opcoes) != 2 || len(got.Descartadas) != 1 || got.Descartadas[0].Servico != "economico" {
			t.Errorf("opções = %+v, descartadas = %+v", got.Opcoes, got.Descartadas)
		}
	})
	t.Run("inflamável é proibido", func(t *testing.T) {
		it := itemBase()
		it.Categoria = "inflamavel"
		if _, e := calcularCotacao(cotacaoLocal(it)); e == nil || e.Codigo != erroItemProibido {
			t.Errorf("erro = %+v", e)
		}
	})
	t.Run("líquido e frágil somam taxas", func(t *testing.T) {
		it := itemBase()
		it.Categoria, it.Fragil = "liquido", true
		got, _ := calcularCotacao(cotacaoLocal(it))
		if o := opcaoDe(t, got, "expresso"); !proximo(o.Preco, 42.2+taxaLiquidoR+taxaFragilR) {
			t.Errorf("expresso = %+v", o)
		}
		if len(got.Avisos) != 2 {
			t.Errorf("avisos = %v, esperado 2", got.Avisos)
		}
	})
}

func TestCalcularCotacaoLimites(t *testing.T) {
	casos := []struct {
		nome   string
		mexe   func(*Item)
		codigo string
	}{
		{"dimensão", func(i *Item) { i.Dimensoes.Altura = 151 }, erroDimensaoExcedida},
		{"peso total", func(i *Item) { i.PesoKg, i.Quantidade = 10, 8 }, erroPesoExcedido},
		{"valor declarado", func(i *Item) { i.ValorUnitario, i.Quantidade = 5000, 3 }, erroValorDeclarado},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			it := itemBase()
			c.mexe(&it)
			if _, e := calcularCotacao(cotacaoLocal(it)); e == nil || e.Codigo != c.codigo {
				t.Errorf("erro = %+v, esperado %s", e, c.codigo)
			}
		})
	}
}

func TestCalcularCotacaoPesoAcimaDe30SemEconomico(t *testing.T) {
	it := itemBase()
	it.PesoKg, it.Quantidade = 10, 4
	got, e := calcularCotacao(cotacaoLocal(it))
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Descartadas) != 1 || got.Descartadas[0].Servico != "economico" {
		t.Errorf("descartadas = %+v", got.Descartadas)
	}
}

func TestCalcularCotacaoPrazoMaximo(t *testing.T) {
	in := cotacaoLocal(itemBase())
	in.PrazoMaximoDias = ptr(2)
	got, _ := calcularCotacao(in)
	if len(got.Opcoes) != 1 || got.Opcoes[0].Servico != "expresso" || len(got.Descartadas) != 2 {
		t.Errorf("opções = %+v, descartadas = %+v", got.Opcoes, got.Descartadas)
	}

	// Prazo que nada atende não é erro: volta vazio, com aviso e motivos.
	in = cotacaoLocal(itemBase())
	in.DestinoCEP = "80000-000"
	in.PrazoMaximoDias = ptr(1)
	got, e := calcularCotacao(in)
	if e != nil || len(got.Opcoes) != 0 || len(got.Descartadas) != 3 || len(got.Avisos) == 0 {
		t.Errorf("got = %+v, erro = %v", got, e)
	}
}

func TestCalcularCotacaoSeguro(t *testing.T) {
	it := itemBase()
	it.ValorUnitario, it.Quantidade = 500, 4 // declarado 2000

	sem, _ := calcularCotacao(cotacaoLocal(it))
	if len(sem.Avisos) != 1 {
		t.Errorf("avisos sem seguro = %v, esperado alerta de valor alto", sem.Avisos)
	}

	in := cotacaoLocal(it)
	in.Seguro = true
	com, _ := calcularCotacao(in)
	o := opcaoDe(t, com, "expresso")
	sem4 := opcaoDe(t, sem, "expresso")
	if !o.SeguroIncluido || !proximo(o.Preco-sem4.Preco, 24) || len(com.Avisos) != 0 {
		t.Errorf("com seguro = %+v (sem = %+v), avisos = %v", o, sem4, com.Avisos)
	}

	// Mínimo de R$ 5 para valores baixos.
	in = cotacaoLocal(itemBase())
	in.Seguro = true
	baixo, _ := calcularCotacao(in)
	if o := opcaoDe(t, baixo, "expresso"); !proximo(o.Preco, 42.2+5) {
		t.Errorf("seguro mínimo: %+v", o)
	}
}

func TestCalcularCotacaoServicosSolicitados(t *testing.T) {
	in := cotacaoLocal(itemBase())
	in.Servicos = []string{"padrao"}
	got, _ := calcularCotacao(in)
	if len(got.Opcoes) != 1 || got.Opcoes[0].Servico != "padrao" || len(got.Descartadas) != 0 {
		t.Errorf("opções = %+v, descartadas = %+v", got.Opcoes, got.Descartadas)
	}
}

func TestCalcularCotacaoSKURepetidoEhAviso(t *testing.T) {
	got, e := calcularCotacao(cotacaoLocal(itemBase(), itemBase()))
	if e != nil || len(got.Avisos) != 1 {
		t.Errorf("avisos = %v, erro = %v", got.Avisos, e)
	}
}

// --- fluxo pelo protocolo MCP ---

func chamarTool(t *testing.T, cs *mcp.ClientSession, nome string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: nome, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool %s: %v", nome, err)
	}
	return res
}

func decodificar[T any](t *testing.T, res *mcp.CallToolResult) T {
	t.Helper()
	if res.IsError {
		t.Fatalf("isError: %+v", res.Content)
	}
	var out T
	bruto, _ := json.Marshal(res.StructuredContent)
	if err := json.Unmarshal(bruto, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func codigoDoErro(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if !res.IsError {
		t.Fatalf("esperava isError, veio %+v", res.StructuredContent)
	}
	var e Erro
	// Erros do schema não têm corpo JSON; devolve vazio nesse caso.
	_ = json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &e)
	return e.Codigo
}

func argsCotacao() map[string]any {
	return map[string]any{
		"origem_cep": "01310-100", "destino_cep": "01999-999",
		"itens": []any{map[string]any{
			"sku": "A1", "quantidade": 1, "peso_kg": 1, "valor_unitario": 100,
			"dimensoes": map[string]any{"comprimento": 10, "largura": 10, "altura": 10},
		}},
	}
}

func argsContratacao(cotacao, chave string, dry any) map[string]any {
	a := map[string]any{
		"cotacao_id": cotacao, "servico": "padrao", "chave_idempotencia": chave,
		"destinatario": map[string]any{"nome": "Maria Silva", "documento": "123.456.789-09"},
		"endereco":     map[string]any{"logradouro": "Rua A", "numero": "10", "cidade": "São Paulo", "uf": "SP"},
	}
	if dry != nil {
		a["dry_run"] = dry
	}
	return a
}

func TestListToolsFrete(t *testing.T) {
	lista, err := conectar(t).ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	porNome := map[string]*mcp.Tool{}
	for _, tool := range lista.Tools {
		porNome[tool.Name] = tool
	}
	if len(porNome) != 2 {
		t.Fatalf("tools = %d, esperado 2", len(porNome))
	}
	contratar := porNome["contratar_envio"]
	if a := contratar.Annotations; a == nil || a.DestructiveHint == nil || !*a.DestructiveHint || !a.IdempotentHint {
		t.Errorf("annotations de contratar_envio = %+v", a)
	}
	if a := porNome["cotar_frete"].Annotations; a == nil || a.DestructiveHint == nil || *a.DestructiveHint {
		t.Errorf("annotations de cotar_frete = %+v", a)
	}
	bruto, _ := json.Marshal(porNome["cotar_frete"].InputSchema)
	var in struct {
		Properties map[string]struct {
			MinItems *int `json:"minItems"`
			MaxItems *int `json:"maxItems"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(bruto, &in); err != nil {
		t.Fatal(err)
	}
	if i := in.Properties["itens"]; i.MinItems == nil || *i.MinItems != 1 || i.MaxItems == nil || *i.MaxItems != 20 {
		t.Errorf("itens = %+v, esperado minItems 1 e maxItems 20", i)
	}
}

func TestFluxoCotarEContratar(t *testing.T) {
	cs := conectar(t)
	cot := decodificar[SaidaCotacao](t, chamarTool(t, cs, "cotar_frete", argsCotacao()))
	if cot.CotacaoID != "COT-000001" || len(cot.Opcoes) != 3 {
		t.Fatalf("cotação = %+v", cot)
	}

	// Sem dry_run informado, o padrão é simular.
	sim := decodificar[SaidaContratacao](t, chamarTool(t, cs, "contratar_envio", argsContratacao(cot.CotacaoID, "pedido-0001", nil)))
	if sim.Status != "simulado" || sim.EnvioID != "" || !proximo(sim.PrecoTotal, 24.6) {
		t.Fatalf("simulação = %+v", sim)
	}

	real := decodificar[SaidaContratacao](t, chamarTool(t, cs, "contratar_envio", argsContratacao(cot.CotacaoID, "pedido-0001", false)))
	if real.Status != "contratado" || real.EnvioID == "" || real.CodigoRastreio == "" || real.Reaproveitado {
		t.Fatalf("contratação = %+v", real)
	}

	// Mesma chave e mesmo pedido: devolve o mesmo envio, sem contratar de novo.
	de_novo := decodificar[SaidaContratacao](t, chamarTool(t, cs, "contratar_envio", argsContratacao(cot.CotacaoID, "pedido-0001", false)))
	if !de_novo.Reaproveitado || de_novo.EnvioID != real.EnvioID {
		t.Errorf("replay = %+v, esperado reaproveitado com envio %s", de_novo, real.EnvioID)
	}

	// Mesma chave com pedido diferente é conflito.
	outro := argsContratacao(cot.CotacaoID, "pedido-0001", false)
	outro["servico"] = "expresso"
	if c := codigoDoErro(t, chamarTool(t, cs, "contratar_envio", outro)); c != erroConflitoIdempotencia {
		t.Errorf("código = %q", c)
	}

	// Chave nova para a mesma cotação: já foi consumida.
	if c := codigoDoErro(t, chamarTool(t, cs, "contratar_envio", argsContratacao(cot.CotacaoID, "pedido-0002", false))); c != erroCotacaoUtilizada {
		t.Errorf("código = %q", c)
	}
}

func TestContratarErros(t *testing.T) {
	cs := conectar(t)
	cot := decodificar[SaidaCotacao](t, chamarTool(t, cs, "cotar_frete", argsCotacao()))

	casos := []struct {
		nome   string
		mexe   func(map[string]any)
		codigo string
	}{
		{"cotação inexistente", func(a map[string]any) { a["cotacao_id"] = "COT-999999" }, erroCotacaoNaoEncontrada},
		{"documento inválido", func(a map[string]any) { a["destinatario"].(map[string]any)["documento"] = "123" }, erroDocumentoInvalido},
		{"e-mail inválido", func(a map[string]any) { a["destinatario"].(map[string]any)["email"] = "sem-arroba" }, erroEmailInvalido},
		{"UF inexistente", func(a map[string]any) { a["endereco"].(map[string]any)["uf"] = "XX" }, erroUFInvalida},
		// Barrados pelo schema antes do handler: voltam isError sem corpo de domínio.
		{"id fora do padrão", func(a map[string]any) { a["cotacao_id"] = "abc" }, ""},
		{"chave curta", func(a map[string]any) { a["chave_idempotencia"] = "x" }, ""},
		{"serviço fora do enum", func(a map[string]any) { a["servico"] = "sedex" }, ""},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			a := argsContratacao(cot.CotacaoID, "chave-teste-1", nil)
			c.mexe(a)
			if got := codigoDoErro(t, chamarTool(t, cs, "contratar_envio", a)); got != c.codigo {
				t.Errorf("código = %q, esperado %q", got, c.codigo)
			}
		})
	}
}

func TestContratarServicoForaDasOpcoes(t *testing.T) {
	cs := conectar(t)
	args := argsCotacao()
	args["servicos"] = []any{"expresso"}
	cot := decodificar[SaidaCotacao](t, chamarTool(t, cs, "cotar_frete", args))
	// argsContratacao pede "padrao", que não foi cotado.
	if c := codigoDoErro(t, chamarTool(t, cs, "contratar_envio", argsContratacao(cot.CotacaoID, "chave-teste-1", nil))); c != erroServicoIndisponivel {
		t.Errorf("código = %q", c)
	}
}

func TestCotarSchemaBarraEntradasInvalidas(t *testing.T) {
	cs := conectar(t)
	casos := map[string]func(map[string]any){
		"CEP com letras":   func(a map[string]any) { a["origem_cep"] = "abcde-123" },
		"sem itens":        func(a map[string]any) { a["itens"] = []any{} },
		"peso zero":        func(a map[string]any) { a["itens"].([]any)[0].(map[string]any)["peso_kg"] = 0 },
		"categoria errada": func(a map[string]any) { a["itens"].([]any)[0].(map[string]any)["categoria"] = "radioativo" },
		"prazo 31":         func(a map[string]any) { a["prazo_maximo_dias"] = 31 },
		"serviço errado":   func(a map[string]any) { a["servicos"] = []any{"sedex"} },
	}
	for nome, mexe := range casos {
		t.Run(nome, func(t *testing.T) {
			a := argsCotacao()
			mexe(a)
			if !chamarTool(t, cs, "cotar_frete", a).IsError {
				t.Error("esperava isError")
			}
		})
	}
}

func TestCotacaoExpira(t *testing.T) {
	a := novoArmazem()
	agora := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
	a.agora = func() time.Time { return agora }

	_, saida, _ := a.cotarFrete(context.Background(), nil, cotacaoLocal(itemBase()))
	cot := saida.(SaidaCotacao)
	if cot.ValidaAte != "2026-11-01T10:15:00Z" {
		t.Errorf("valida_ate = %s", cot.ValidaAte)
	}

	in := EntradaContratacao{
		CotacaoID: cot.CotacaoID, Servico: "padrao", ChaveIdempotencia: "chave-teste-1",
		Destinatario: Destinatario{Nome: "Maria", Documento: "12345678909"},
		Endereco:     Endereco{Logradouro: "Rua A", Numero: "1", Cidade: "SP", UF: "SP"},
	}
	if res, _, _ := a.contratarEnvio(context.Background(), nil, in); res != nil {
		t.Fatalf("dentro da validade deveria passar: %+v", res.Content)
	}

	agora = agora.Add(validadeCotacao + time.Second)
	res, _, _ := a.contratarEnvio(context.Background(), nil, in)
	if res == nil || !res.IsError {
		t.Fatal("esperava cotacao_expirada")
	}
	var e Erro
	_ = json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &e)
	if e.Codigo != erroCotacaoExpirada {
		t.Errorf("erro = %+v", e)
	}
}
