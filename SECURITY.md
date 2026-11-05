# Segurança

Este é um projeto de laboratório/template. Antes de usar em produção:

- Não exponha o modo HTTP (`-http`) diretamente; coloque autenticação e rate limit na frente (ver `deploy/kong.yml`).
- Troque a chave `poc-chave-local` e nunca versione segredos reais.
- Persista o estado fora da memória e aplique TTL às cotações.
- Revise o que as tools logam (hoje: sem dados pessoais do destinatário).

Para reportar uma vulnerabilidade, abra um *private security advisory* no repositório.
