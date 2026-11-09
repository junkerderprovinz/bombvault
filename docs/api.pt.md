# API e integrações

O BombVault tem uma pequena API HTTP para scripts, painéis e domótica. Lê o mesmo que a visão geral mostra e pode iniciar um backup. Tudo o resto, como restauros, eliminação de backups e definições, fica na interface web.

## Tokens {#tokens}

Cada pedido precisa de um token da API, mesmo sem palavra-passe de acesso. Cria-o em **Definições, Integrações, Tokens da API**:

1. Escreve um nome que diga onde o token é usado, por exemplo "Home Assistant" ou "Uptime Kuma".
2. Liga **Permitir iniciar backups** se o token deve poder iniciar backups. Sem isso só pode ler.
3. Clica em **Criar token**. O token aparece uma única vez. O BombVault só guarda uma impressão dele, por isso copia-o agora.

Envia o token num cabeçalho, `Authorization: Bearer <token>` ou `X-API-Key: <token>`. Um token começa por `bvapi_`. Só abre a API: uma chave MCP não funciona aqui, e um token não funciona para MCP.

Cada token tem um mosaico com o nome, se pode iniciar backups, os últimos quatro caracteres, quando e de onde foi usado pela última vez e as chamadas de hoje. No mosaico podes mudar-lhe o nome, alterar o que pode fazer, substituí-lo ou revogá-lo. **Registo** mostra os backups que iniciou e as últimas chamadas. Restaurar a configuração do BombVault a partir de um backup revoga todos os tokens, porque o backup pode conter tokens que revogaste depois.

Sem palavra-passe de acesso, quem conseguir abrir a interface web também consegue criar um token. Se abrires o BombVault com um nome que parece público e sem palavra-passe, não se criam tokens a partir desse endereço, tal como com as [chaves MCP](mcp.md#switch-on).

## Rotas {#endpoints}

| Rota | O que devolve ou faz | Token |
|---|---|---|
| `GET /api/v1/health` | Versão, nome da instância, se há um backup em curso e o que este token pode fazer | leitura |
| `GET /api/v1/status` | Estado de proteção por secção: último backup bem-sucedido, intervalo esperado, verificações, próximas execuções | leitura |
| `GET /api/v1/activity` | O que está a correr agora, com fase e percentagem | leitura |
| `GET /api/v1/items` | Cada entrada protegida com o agendamento, o que um backup para e o último backup. `?domain=` para uma secção | leitura |
| `GET /api/v1/runs` | Histórico de execuções, as mais recentes primeiro. Filtros `limit`, `domain`, `item`, `status`, `kind`, `since` | leitura |
| `GET /api/v1/anomalies` | Anomalias com um resumo do que está aberto. Filtros `state`, `severity`, `domain`, `limit` | leitura |
| `GET /api/v1/anomalies/{id}` | Uma anomalia | leitura |
| `GET /api/v1/storage/{domain}` | Histórico de tamanho, crescimento semanal e espaço livre de cada repositório de uma secção | leitura |
| `POST /api/v1/backups` | Faz o backup de uma entrada (`{"domain":"containers","item":"plex"}`) ou de uma secção inteira (`{"domain":"vms"}`) | início |
| `POST /api/v1/backups/everything` | Executa o Backup total | início |
| `POST /api/v1/runs/{id}/cancel` | Cancela um backup em curso iniciado por este token | início |

As secções são `containers`, `vms`, `files`, `zfs`, `flash` e `config`. As horas são segundos Unix. As respostas são as das [ferramentas MCP](mcp.md#tools) com o mesmo nome, por isso as duas andam a par.

Um backup iniciado aqui é o mesmo que a interface web inicia: um contentor em execução para até o backup terminar. O pedido volta de imediato, e `/api/v1/activity` e `/api/v1/runs` mostram como corre.

## Exemplos {#examples}

```sh
# Como estão os backups?
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
| 403 | `not_permitted`, `forbidden_origin` | O token só pode ler, ou não iniciou essa execução, ou o pedido veio de uma página de outra origem |
| 404 | `not_found` | Não existe essa entrada, execução ou anomalia |
| 409 | `busy`, `domain_off`, `nothing_to_back_up`, `not_running` | Já há algo a correr, a secção está desligada ou não há nada a fazer |
| 429 | `throttled`, `rate_limited`, `cooldown`, `retention_guard` | Um limite retém o pedido. `Retry-After` diz quando tentar de novo |

Os inícios seguem os mesmos limites dos [inícios por MCP](mcp.md#starting-backups): 12 por hora e por token, 15 minutos entre dois inícios da mesma entrada, no máximo 4 inícios de uma entrada em 24 horas e a proteção da retenção. Os três últimos contam juntos os inícios por MCP, pela API e a partir do Home Assistant. Um token pode fazer 120 pedidos por minuto. Cinco tentativas falhadas a partir de um endereço bloqueiam-no durante um minuto.

## OpenAPI {#openapi}

O BombVault serve uma descrição destas rotas em `/api/v1/openapi.json` (OpenAPI 3.1). Não precisa de token. Carrega-a no Swagger UI, no Postman ou num gerador de código.

## Home Assistant {#home-assistant}

O BombVault pode aparecer no Home Assistant como um dispositivo, através da descoberta MQTT. O Home Assistant precisa da sua integração MQTT e de um broker, por exemplo o add-on Mosquitto. Não é preciso nenhum componente próprio.

1. No BombVault, abre **Definições, Integrações** e liga **Home Assistant** no cartão **Serviços**. Os campos do broker aparecem por baixo.
2. Escreve o endereço e a porta do broker, e o utilizador e a palavra-passe se os pedir. Liga **Usar TLS** se o broker usar TLS, normalmente na porta 8883. O certificado tem de ser válido para o endereço que escreveste. Se mudares o endereço, a porta ou o nome de utilizador, volta a introduzir a palavra-passe: o BombVault não passa a guardada a outro broker ou utilizador.
3. Clica em **Guardar**. A linha mostra quando a ligação está estabelecida.

O dispositivo chama-se BombVault, ou BombVault com o nome da instância entre parênteses, e tem estas entidades:

| Entidade | O que mostra |
|---|---|
| Status | `ok`, `warning`, `overdue`, `failed` ou `off`, o pior entre as secções ligadas. Uma secção à espera do primeiro backup só conta depois de esse backup estar previsto, e só uma secção em dia dá o estado `ok`. `overdue` significa, como na vista geral, que o último backup de uma secção tem mais de dois intervalos |
| Running job | O que está a correr agora, ou `idle` |
| Open anomalies | Quantas anomalias estão abertas |
| Next scheduled backup | Quando começa o próximo backup agendado |
| *Secção* last backup | Quando correu o último backup bem-sucedido da secção |
| *Secção* last result | Como terminou o seu último backup |
| *Secção* repository free space | O espaço livre onde está o seu repositório principal, se o BombVault o conseguir ler |
| Back up *secção* | Um botão que faz o backup da secção inteira |

Cada secção ligada tem as suas entidades, e uma secção que desligues perde-as. Os botões aparecem quando ligas **Os botões iniciam backups**, que vem desligado numa instalação nova. Seguem os mesmos limites dos [inícios pela API](#errors). Além disso, o BombVault aceita um só toque de cada vez por secção e no máximo seis por minuto, e ignora um toque que o broker guardou como mensagem retida (retained). Qualquer pessoa que consiga publicar no broker pode carregar neles, por isso protege o broker com uma palavra-passe.

O BombVault lê o seu estado a cada 15 segundos e publica-o quando algo mudou, como JSON em `<prefixo>/<nó>/state`. O prefixo é `bombvault` enquanto não o mudares, e o nó é um identificador curto que o BombVault escolhe uma vez. As mensagens de descoberta vão para o prefixo predefinido do Home Assistant, `homeassistant`. Ambos ficam retidos (retained). Uma última mensagem (last will) marca o dispositivo como indisponível se o BombVault parar sem avisar. Desligar a ligação remove o dispositivo e as suas entidades do Home Assistant.

### Exemplo: um cartão e um alerta {#home-assistant-example}

O Home Assistant cria os ids das entidades a partir dos nomes, por isso o dispositivo BombVault recebe ids como `sensor.bombvault_status`. Com um nome de instância entre parênteses, os ids levam também esse nome, por isso confirma-os primeiro em **Developer Tools, States**. Um cartão para o painel:

```yaml
type: entities
title: Backups
entities:
  - sensor.bombvault_status
  - sensor.bombvault_running_job
  - sensor.bombvault_open_anomalies
  - sensor.bombvault_next_scheduled_backup
```

Uma automação que avisa o teu telemóvel quando o estado passa a `failed` ou `overdue`:

```yaml
alias: BombVault needs you
trigger:
  - platform: state
    entity_id: sensor.bombvault_status
    to:
      - failed
      - overdue
action:
  - service: notify.mobile_app_my_phone
    data:
      message: "BombVault status: {{ trigger.to_state.state }}. Open the overview."
```

## Prometheus {#metrics}

**Expor /metrics** no cartão **Serviços** em **Definições, Integrações** serve o estado dos backups, tamanhos e datas para o Prometheus, o Grafana ou o Uptime Kuma, em `/metrics` na porta da interface web. Vem desligado por predefinição. O **Token de scrape (opcional)** faz o scraper enviar um bearer token. As etiquetas não trazem segredos nem caminhos.

## Widget de painel {#dashboard-widget}

O cartão **Widget de painel** em **Definições, Integrações** gera um token só de leitura e dá-te um endereço para qualquer painel que mostre um iframe, como o Homepage, o Organizr ou o Heimdall. O endereço abre uma pequena página em `/widget` só com o registo de atividade em direto. O token dá acesso a esse registo e a mais nada, e **Desativar** revoga-o de imediato. A página incorporada só existe em inglês.

## Encontrar o BombVault na rede {#mdns}

O BombVault anuncia a sua interface web na rede local por mDNS, o protocolo por trás do Bonjour e do Avahi. Um navegador chega-lhe então como `https://bombvault.local:3443`, ou `http://bombvault.local:3000` com `HTTP_ONLY`, e os navegadores de serviços mostram-no como serviço web com o subtipo `_bombvault`. Os seus registos TXT levam a versão e o caminho. O interruptor é **Encontrar na rede**, no cartão **Serviços** em **Definições, Integrações**, e vem ligado. Se outro dispositivo já usar o nome, o BombVault fica com `bombvault-2.local` e assim por diante, e o cartão mostra o endereço que obteve. Quando o BombVault para ou desligas o anúncio, avisa a rede, e os navegadores retiram a entrada de imediato.

Se o anúncio chega à tua rede depende de como o contentor está ligado:

- **bridge**, o predefinido no modelo do Unraid: o anúncio fica dentro da rede do Docker e ninguém na rede local o vê. Abre o BombVault pelo endereço do anfitrião como até agora.
- **br0** ou outra rede macvlan ou ipvlan: o contentor tem o seu próprio endereço na rede local e o anúncio chega lá.
- **host**: o anúncio sai pelas interfaces do anfitrião, ao lado do anúncio do próprio Unraid. As bridges do Docker e do libvirt ficam de fora.

Só são anunciados endereços IPv4.
