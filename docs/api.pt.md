# API e integrações

O BombVault tem uma pequena API HTTP para scripts, painéis e domótica. Lê o mesmo que o painel mostra e pode iniciar uma cópia. Tudo o resto, como restauros, eliminação de cópias e definições, fica na interface web.

## Tokens {#tokens}

Cada pedido precisa de um token da API, mesmo sem palavra-passe de acesso. Cria-o em **Definições, Sistema, Tokens da API**:

1. Escreve um nome que diga onde o token é usado, por exemplo "Home Assistant" ou "Uptime Kuma".
2. Liga **Permitir iniciar cópias** se o token deve poder iniciar cópias. Sem isso só pode ler.
3. Clica em **Criar token**. O token aparece uma única vez. O BombVault só guarda uma impressão dele, por isso copia-o agora.

Envia o token num cabeçalho, `Authorization: Bearer <token>` ou `X-API-Key: <token>`. Um token começa por `bvapi_`. Só abre a API: uma chave MCP não funciona aqui, e um token não funciona para MCP.

Cada token tem um mosaico com o nome, se pode iniciar cópias, os últimos quatro caracteres, quando e de onde foi usado pela última vez e as chamadas de hoje. No mosaico podes mudar-lhe o nome, alterar o que pode fazer, substituí-lo ou revogá-lo. **Registo** mostra as cópias que iniciou e as últimas chamadas. Restaurar a configuração do BombVault a partir de uma cópia revoga todos os tokens, porque a cópia pode conter tokens que revogaste depois.

Sem palavra-passe de acesso, quem conseguir abrir a interface web também consegue criar um token. Se abrires o BombVault com um nome que parece público e sem palavra-passe, não se criam tokens a partir desse endereço, tal como com as [chaves MCP](mcp.md#switch-on).

## Rotas {#endpoints}

| Rota | O que devolve ou faz | Token |
|---|---|---|
| `GET /api/v1/health` | Versão, nome da instância, se há uma cópia em curso e o que este token pode fazer | leitura |
| `GET /api/v1/status` | Estado de proteção por domínio: última cópia bem-sucedida, intervalo esperado, verificações, próximas execuções | leitura |
| `GET /api/v1/activity` | O que está a correr agora, com fase e percentagem | leitura |
| `GET /api/v1/items` | Cada item protegido com o agendamento, o que uma cópia para e a última cópia; `?domain=` para um domínio | leitura |
| `GET /api/v1/runs` | Histórico de execuções, as mais recentes primeiro; filtros `limit`, `domain`, `item`, `status`, `kind`, `since` | leitura |
| `GET /api/v1/anomalies` | Anomalias com um resumo do que está aberto; filtros `state`, `severity`, `domain`, `limit` | leitura |
| `GET /api/v1/anomalies/{id}` | Uma anomalia | leitura |
| `GET /api/v1/storage/{domain}` | Histórico de tamanho, crescimento semanal e espaço livre de cada repositório de um domínio | leitura |
| `POST /api/v1/backups` | Copia um item (`{"domain":"containers","item":"plex"}`) ou um domínio inteiro (`{"domain":"vms"}`) | início |
| `POST /api/v1/backups/everything` | Executa o Backup Everything | início |
| `POST /api/v1/runs/{id}/cancel` | Cancela uma cópia em curso iniciada por este token | início |

Os domínios são `containers`, `vms`, `files`, `zfs`, `flash` e `config`. As horas são segundos Unix. As respostas são as das [ferramentas MCP](mcp.md#tools) com o mesmo nome, por isso as duas andam a par.

Uma cópia iniciada aqui é a mesma que a interface web inicia: um contentor em execução para até a cópia terminar. O pedido volta de imediato, e `/api/v1/activity` e `/api/v1/runs` mostram como corre.

## Exemplos {#examples}

```sh
# Como estão as cópias?
curl -s -H "Authorization: Bearer $BOMBVAULT_TOKEN" https://tower:3443/api/v1/status

# Copiar um contentor agora.
curl -s -X POST -H "Authorization: Bearer $BOMBVAULT_TOKEN" \
  -H "Content-Type: application/json" -d '{"domain":"containers","item":"plex"}' \
  https://tower:3443/api/v1/backups
```

Com o certificado autoassinado do BombVault, junta `--cacert bombvault-cert.pem` (o ficheiro que obténs em **Descarregar certificado** no cartão MCP) ou `-k` numa rede de confiança.

## Erros e limites {#errors}

Um erro volta como `{"error": {"code": "...", "message": "..."}}` com o estado correspondente:

| Estado | Códigos | Significado |
|---|---|---|
| 400 | `invalid_argument`, `ambiguous` | Falta um argumento ou está errado |
| 401 | `no_token`, `invalid_token` | Sem token, ou não é um token ativo |
| 403 | `not_permitted` | O token só pode ler, ou não iniciou essa execução |
| 404 | `not_found` | Não existe esse item, execução ou anomalia |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Já há algo a correr, o domínio está desligado ou não há nada a fazer |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | Um limite retém o pedido; `Retry-After` diz quando tentar de novo |

Os inícios seguem os mesmos limites dos [inícios por MCP](mcp.md#starting-backups): 12 por hora e por token, 15 minutos entre dois inícios do mesmo item, no máximo 4 inícios de um item em 24 horas e a proteção da retenção. Os três últimos contam juntos os inícios por MCP e pela API. Um token pode fazer 120 pedidos por minuto. Cinco tentativas falhadas a partir de um endereço bloqueiam-no durante um minuto.

## OpenAPI {#openapi}

O BombVault serve uma descrição destas rotas em `/api/v1/openapi.json` (OpenAPI 3.1). Não precisa de token. Carrega-a no Swagger UI, no Postman ou num gerador de código.
