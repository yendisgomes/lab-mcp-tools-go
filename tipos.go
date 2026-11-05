package main

type Dimensoes struct {
	Comprimento float64 `json:"comprimento" jsonschema:"comprimento de UMA unidade, em cm"`
	Largura     float64 `json:"largura" jsonschema:"largura de UMA unidade, em cm"`
	Altura      float64 `json:"altura" jsonschema:"altura de UMA unidade, em cm"`
}

type Item struct {
	SKU           string    `json:"sku" jsonschema:"código do produto"`
	Quantidade    int       `json:"quantidade" jsonschema:"unidades deste SKU (1 a 100)"`
	PesoKg        float64   `json:"peso_kg" jsonschema:"peso de UMA unidade, em kg"`
	Dimensoes     Dimensoes `json:"dimensoes" jsonschema:"dimensões da embalagem de UMA unidade"`
	ValorUnitario float64   `json:"valor_unitario" jsonschema:"valor de UMA unidade, em reais"`
	Fragil        bool      `json:"fragil,omitempty" jsonschema:"true se o item é frágil (taxa de manuseio por linha)"`
	Categoria     string    `json:"categoria,omitempty" jsonschema:"comum (padrão), perecivel, liquido, bateria ou inflamavel"`
}

type EntradaCotacao struct {
	OrigemCEP       string   `json:"origem_cep" jsonschema:"CEP de origem, com 8 dígitos (hífen opcional)"`
	DestinoCEP      string   `json:"destino_cep" jsonschema:"CEP de destino, com 8 dígitos (hífen opcional)"`
	Itens           []Item   `json:"itens" jsonschema:"itens do pacote (1 a 20 linhas)"`
	Servicos        []string `json:"servicos,omitempty" jsonschema:"serviços a cotar; omitido cota todos"`
	Seguro          bool     `json:"seguro,omitempty" jsonschema:"true para incluir seguro (1,2% do valor declarado, mínimo R$ 5)"`
	PrazoMaximoDias *int     `json:"prazo_maximo_dias,omitempty" jsonschema:"descarta opções mais lentas que este prazo (1 a 30)"`
}

type Opcao struct {
	Servico        string  `json:"servico" jsonschema:"economico, padrao ou expresso"`
	Preco          float64 `json:"preco" jsonschema:"preço total em reais, já com taxas e seguro"`
	PrazoDias      int     `json:"prazo_dias" jsonschema:"prazo de entrega em dias corridos"`
	SeguroIncluido bool    `json:"seguro_incluido" jsonschema:"se o preço inclui seguro"`
}

type Descarte struct {
	Servico string `json:"servico" jsonschema:"serviço que não foi oferecido"`
	Motivo  string `json:"motivo" jsonschema:"por que foi descartado"`
}

type SaidaCotacao struct {
	CotacaoID      string     `json:"cotacao_id" jsonschema:"identificador a passar para contratar_envio"`
	ValidaAte      string     `json:"valida_ate" jsonschema:"fim da validade da cotação (RFC 3339)"`
	Zona           string     `json:"zona" jsonschema:"local, regional ou nacional, pelo primeiro dígito dos CEPs"`
	PesoTaxadoKg   float64    `json:"peso_taxado_kg" jsonschema:"maior entre peso real e cubado, somando todas as unidades"`
	ValorDeclarado float64    `json:"valor_declarado" jsonschema:"soma de valor_unitario × quantidade"`
	Opcoes         []Opcao    `json:"opcoes" jsonschema:"opções disponíveis, vazio se nenhuma atende"`
	Descartadas    []Descarte `json:"descartadas" jsonschema:"serviços não oferecidos e o motivo"`
	Avisos         []string   `json:"avisos" jsonschema:"alertas que o chamador deve considerar"`
}

type Destinatario struct {
	Nome      string `json:"nome" jsonschema:"nome completo"`
	Documento string `json:"documento" jsonschema:"CPF (11 dígitos) ou CNPJ (14), com ou sem pontuação"`
	Telefone  string `json:"telefone,omitempty" jsonschema:"telefone para contato"`
	Email     string `json:"email,omitempty" jsonschema:"e-mail para notificações"`
}

type Endereco struct {
	Logradouro  string `json:"logradouro" jsonschema:"rua, avenida etc."`
	Numero      string `json:"numero" jsonschema:"número ou s/n"`
	Complemento string `json:"complemento,omitempty" jsonschema:"apartamento, bloco etc."`
	Cidade      string `json:"cidade" jsonschema:"cidade de entrega"`
	UF          string `json:"uf" jsonschema:"sigla do estado, 2 letras"`
}

type EntradaContratacao struct {
	CotacaoID         string       `json:"cotacao_id" jsonschema:"cotacao_id devolvido por cotar_frete"`
	Servico           string       `json:"servico" jsonschema:"economico, padrao ou expresso (deve estar nas opções da cotação)"`
	Destinatario      Destinatario `json:"destinatario" jsonschema:"quem recebe"`
	Endereco          Endereco     `json:"endereco" jsonschema:"endereço de entrega"`
	ChaveIdempotencia string       `json:"chave_idempotencia" jsonschema:"chave única por envio (8 a 64 caracteres); repetir a chave com o mesmo pedido devolve o mesmo envio"`
	DryRun            *bool        `json:"dry_run,omitempty" jsonschema:"padrão true: só valida e simula; false contrata de verdade"`
}

type SaidaContratacao struct {
	Status          string  `json:"status" jsonschema:"simulado ou contratado"`
	EnvioID         string  `json:"envio_id,omitempty" jsonschema:"só quando contratado"`
	CodigoRastreio  string  `json:"codigo_rastreio,omitempty" jsonschema:"só quando contratado"`
	Servico         string  `json:"servico" jsonschema:"serviço escolhido"`
	PrecoTotal      float64 `json:"preco_total" jsonschema:"valor cobrado, em reais"`
	PrevisaoEntrega string  `json:"previsao_entrega" jsonschema:"data prevista (AAAA-MM-DD)"`
	Reaproveitado   bool    `json:"reaproveitado" jsonschema:"true se a chave de idempotência já tinha esse envio"`
	Texto           string  `json:"texto" jsonschema:"resumo legível em uma linha"`
}
