---
name: frete-envio
description: Use quando o usuário pedir para cotar frete, comparar prazos e preços de envio, ou contratar/despachar um pacote. Orienta o fluxo das tools cotar_frete e contratar_envio do MCP mcp-frete, incluindo o que perguntar, o que assumir e quando confirmar antes de contratar.
---

# Frete e envio

Duas tools do servidor `mcp-frete`, sempre nesta ordem: `cotar_frete`, depois `contratar_envio`. Nunca calcule preço ou prazo de cabeça, nunca invente `cotacao_id`.

## Fluxo

1. Reúna os dados e cote com `cotar_frete`.
2. Apresente as opções e deixe o usuário escolher.
3. Chame `contratar_envio` com `dry_run: true` (ou omitido) e mostre o resumo.
4. **Só com um "sim" explícito** do usuário a esta contratação, chame de novo com `dry_run: false` e a **mesma** `chave_idempotencia`.

Contratar é irreversível. "Pode cotar" ou "qual o melhor frete" nunca autoriza contratar. Confirmação dada para uma cotação não vale para outra.

## Dados para cotar

**Pergunte** (não adivinhe) quando faltar: CEP de origem ou destino, peso de cada item, dimensões de cada item, quantidade, valor.

**Normalize** antes de chamar:
- CEP: 8 dígitos, com ou sem hífen.
- Peso em kg ("500 g" → `0.5`), dimensões em cm ("1,2 m" → `120`), valor em reais.
- `peso_kg`, `dimensoes` e `valor_unitario` são de **uma unidade**; a quantidade vai em `quantidade`. Não multiplique você mesmo.
- Mesmo produto em duas linhas: some em uma linha só (a tool avisa se repetir o SKU).

**`categoria`**: omita quando for um produto comum. Infira pelo que o usuário descreveu, e pergunte se ficar em dúvida:
- comida fresca, flores, remédio refrigerado → `perecivel`
- bebida, perfume, tinta, produto de limpeza → `liquido`
- celular, notebook, power bank, qualquer coisa com bateria de lítio → `bateria`
- álcool, solvente, aerossol, gás → `inflamavel` (a tool recusa; veja Erros)

**`fragil`**: `true` para vidro, cerâmica, eletrônicos e o que o usuário chamar de frágil. Se for ambíguo, pergunte; a taxa é cobrada por linha.

**`seguro`**: se o valor declarado (valor × quantidade, somado) passar de R$ 1.000, a tool avisa. Nesse caso, pergunte se o usuário quer seguro e cote com `seguro: true` se sim. Abaixo disso, não ofereça por conta própria.

**`servicos`**: omita para comparar todos. Passe só os que o usuário pediu ("só expresso").

**`prazo_maximo_dias`**: só quando o usuário der um prazo ("preciso em 3 dias" → `3`). Nunca invente.

## Escolher a opção

- "mais barato" → menor `preco`. "mais rápido" → menor `prazo_dias`.
- Sem critério: mostre as opções em tabela (serviço, preço, prazo) e pergunte.
- Preços e prazos vêm sempre de `opcoes`. Se um serviço está em `descartadas`, explique o `motivo` em linguagem simples; não ofereça esse serviço.
- Leia os `avisos` e repasse os que afetam o preço ou a decisão (taxas, seguro recomendado).
- A cotação vale até `valida_ate` (15 min). Passou disso, cote de novo e **reconfirme o preço** se mudou.

## Cotação sem opções

`opcoes` vazio não é erro: todo serviço foi descartado. Mostre os motivos e proponha uma saída, sem aplicar sozinho:
- prazo apertado demais → "sem o limite de prazo, o padrão sai por R$ X em Y dias; quer assim?"
- peso/volume → dividir em pacotes.
Só cote de novo com o parâmetro relaxado depois que o usuário concordar.

## Contratar

**Destinatário e endereço**: peça tudo que faltar (nome, documento, logradouro, número, cidade, UF). Nunca invente nem complete dados pessoais. `documento` é CPF (11 dígitos) ou CNPJ (14).

**`chave_idempotencia`**: gere uma chave única por envio (ex.: `envio-` + data + sufixo curto, 8 a 64 caracteres) e guarde-a na conversa.
- Mesma chave na simulação, na contratação e em qualquer retry do **mesmo** pedido. Se a chamada deu timeout, repita com a mesma chave; a tool devolve o envio original (`reaproveitado: true`) em vez de duplicar.
- Chave **nova** para outro envio, mesmo que o conteúdo seja parecido.

**Antes do `dry_run: false`**, mostre ao usuário: serviço, preço total, previsão de entrega, destinatário e endereço, e pergunte "posso contratar?".

## Erros

O corpo vem em JSON: `{"erro": código, "mensagem": ..., "sugestao": ...}`. Siga a `sugestao`.

| Código | O que fazer |
|---|---|
| `item_proibido` | Explique que o item não pode ser enviado; ofereça cotar o resto do pacote sem ele |
| `peso_excede_limite`, `valor_declarado_excede_limite` | Proponha dividir em pacotes e cote cada um |
| `dimensao_excede_limite` | Confirme as medidas; se estiverem certas, o item não segue |
| `perecivel_fora_da_regiao` | Explique a restrição regional; não tente outro serviço |
| `cep_invalido` | Peça o CEP de novo |
| `cotacao_expirada` | Cote de novo e reconfirme o preço |
| `cotacao_nao_encontrada` | Cote de novo; não reutilize ids de outras conversas |
| `cotacao_ja_utilizada` | Já houve um envio com essa cotação; para outro envio, cote de novo |
| `servico_indisponivel_na_cotacao` | Mostre as opções da cotação e peça nova escolha |
| `documento_invalido`, `email_invalido`, `uf_invalida` | Peça o dado correto ao usuário |
| `conflito_idempotencia` | Você reutilizou uma chave em outro pedido. Gere uma chave nova **e confirme com o usuário** antes de tentar de novo |

Se a chamada voltar erro sem esse JSON (ex.: validação do schema), leia a mensagem, corrija o argumento e tente uma vez. Se repetir, pare e conte ao usuário o que aconteceu.

## Resposta

Seja curto. Para cotação, uma tabela de opções e uma recomendação de uma linha. Para contratação, informe `envio_id`, `codigo_rastreio`, preço e previsão. Nunca diga que contratou se `status` for `simulado`.
