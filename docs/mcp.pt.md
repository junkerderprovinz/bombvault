# Servidor MCP

O BombVault traz um servidor para o Model Context Protocol (MCP), o protocolo com que assistentes de IA como o Claude Code e o Claude Desktop chegam a ferramentas externas. Através dele, um assistente pode ler como estão as suas cópias e, se o permitir, iniciar uma cópia ou cancelar uma que ele próprio iniciou. Fica desligado até criar uma chave: sem uma chave ativa, o ponto de ligação `/mcp` responde `404` a tudo.

## O que um assistente pode e não pode fazer {#tools}

| Ferramenta | O que faz | Tipo |
|---|---|---|
| `get_health` | Versão, nome da instância, se há uma cópia em curso e o que esta chave pode fazer | leitura |
| `get_status` | Estado de proteção por domínio: última cópia bem-sucedida, intervalo esperado, verificações e controlos externos, próximas execuções agendadas | leitura |
| `get_coverage` | O que o BombVault protege e o que não protege, com o motivo de cada caso | leitura |
| `list_items` | Cada contentor, VM e conjunto de pastas protegido, a pen flash e a configuração da app, com o agendamento, o que uma cópia para, a última cópia e quanto demorou; os contentores de base de dados indicam também o último dump; os datasets ZFS também aparecem, com o resultado da sua última verificação | leitura |
| `list_runs` | Histórico de execuções, as mais recentes primeiro, filtrável por domínio, elemento, estado, tipo e data | leitura |
| `list_restore_points` | Pontos de restauro de um elemento no seu repositório principal e, para um contentor, os seus dumps de base de dados; um dataset ZFS tem um ponto de restauro por cópia, com um snapshot de cada dataset abaixo dele | leitura |
| `get_activity` | O que está a correr agora, com fase e percentagem | leitura |
| `get_storage_stats` | Histórico de tamanho do repositório principal de um domínio e o seu crescimento semanal | leitura |
| `list_anomalies` | Anomalias que o BombVault detetou nas cópias, filtráveis por estado, gravidade e domínio, com um resumo do que está em aberto | leitura |
| `get_anomaly` | Uma dessas ocorrências, com a nota deixada quando foi reconhecida | leitura |
| `start_backup` | Faz já a cópia de um elemento | início |
| `start_domain_backup` | Faz a cópia de cada elemento protegido de um domínio | início |
| `start_backup_everything` | Corre a passagem Backup Everything | início |
| `cancel_backup` | Cancela uma cópia em curso que esta chave iniciou | cancelamento |

Fica na interface web: os restauros de qualquer tipo (incluindo descarregar, guardar ou importar um dump de base de dados), apagar cópias, prune, unlock, as verificações e os exercícios, a replicação externa, as definições, as credenciais e as chaves MCP, e cancelar uma cópia iniciada pelo agendamento, pela interface web ou por outra chave. O mesmo vale para reconhecer uma anomalia ou marcá-la como esperada, o que se faz na página **Anomalias**. O motivo: as respostas das ferramentas contêm nomes e mensagens de erro do seu servidor, e qualquer um deles pode trazer texto escrito para manipular o assistente. Um assistente que caia nesse texto pode, no pior caso, iniciar uma cópia dentro dos limites abaixo ou cancelar uma que ele próprio iniciou.

Se o repositório principal de um elemento for remoto (S3, REST, SFTP, rclone), `list_restore_points` contacta-o e a chamada pode demorar algum tempo. As cópias externas não podem ser listadas por MCP. O que as verificações de anomalias analisam está descrito em [Funcionalidades](features.md), e como um elemento ZFS guarda um snapshot por conjunto de dados, em [Conjuntos de dados ZFS](zfs-datasets.md#contents).

## O que faz uma cópia iniciada {#starting-backups}

A cópia de um assistente é a mesma que a interface web inicia. Um contentor em execução é parado até a sua cópia terminar, junto com os contentores configurados para parar com ele. Uma VM com o método "graceful" é desligada e arrancada de novo. Um dataset ZFS para os contentores configurados para ele enquanto o seu snapshot é criado. Os conjuntos de pastas, a pen flash e a configuração continuam a correr. Depois, o BombVault aplica a política de retenção e pode copiar para o repositório externo. `list_items` diz ao assistente o que um elemento para e quanto demorou a sua última cópia, e as descrições das ferramentas pedem-lhe que lho diga antes de iniciar o que quer que seja.

Como uma cópia para serviços e faz sair pontos de restauro antigos, os inícios por MCP são limitados:

- 12 cópias iniciadas por hora e por chave.
- 15 minutos entre dois inícios MCP do mesmo elemento, do mesmo domínio ou de Backup Everything.
- No máximo 4 inícios MCP do mesmo elemento em 24 horas.
- **Proteção da retenção.** Quando um domínio mantém um número fixo de pontos de restauro (só "manter os últimos N", sem regra diária, semanal ou mensal, localmente ou num destino externo), cada cópia nova faz sair a mais antiga. O BombVault recusa então um início MCP de um elemento cujas N-1 cópias bem-sucedidas mais recentes foram todas iniciadas por MCP. Assim fica sempre no conjunto mantido pelo menos um ponto de restauro criado pelo agendamento ou por si. Com "manter o último" (N = 1), um assistente não consegue fazer cópia desse elemento. A próxima cópia agendada volta a abrir espaço.

Um início de domínio ou de Backup Everything deixa de fora os elementos que um limite retém e indica-os na resposta. A interface web e o agendamento não são afetados por nada disto. A quota horária vive em memória, por isso um reinício do BombVault repõe-na a zero.

## Ativar {#switch-on}

1. Abra **Definições, Sistema, Servidor MCP** e clique em **Chave nova**.
2. Dê à chave um nome que diga onde é usada, por exemplo "Claude Code no portátil". Com uma chave por cliente pode revogar uma sem mexer nas outras.
3. Deixe **Permitir iniciar cópias** ligado, ou desligue-o para uma chave que só deve ler. Pode mudar isto mais tarde no mosaico da chave, e a mudança vale a partir do pedido seguinte do assistente, sem nova ligação.
4. Clique em **Criar chave**. A chave aparece uma única vez. O BombVault guarda só uma impressão digital dela e não a pode mostrar de novo, por isso copie-a já ou use um dos excertos abaixo, que passam a conter a chave real.

Sem palavra-passe de início de sessão, a própria interface web está aberta a toda a sua rede, e quem a conseguir abrir também pode criar uma chave. O cartão avisa disso. Se abrir o BombVault com um nome que parece público (por exemplo `bombvault.example.com` atrás de um proxy inverso) e não houver palavra-passe de início de sessão, a partir desse endereço não é possível criar nem substituir chaves, para que nenhuma página da Internet consiga levar o seu navegador a criar uma. Defina uma palavra-passe de início de sessão, ou abra o BombVault pelo endereço IP ou por um nome local como `tower` ou `tower.local`.

## As suas chaves e o registo delas {#keys}

Cada chave tem o seu próprio mosaico no cartão. Mostra o nome da chave, se pode iniciar cópias ou só ler, os quatro últimos caracteres da chave, quando foi criada ou substituída pela última vez, quando um cliente a usou pela última vez e quantas chamadas fez hoje. No mosaico muda o nome da chave, altera a permissão, substitui-a ou revoga-a. Uma chave revogada passa para a lista de chaves revogadas, onde a pode apagar de vez quando nenhuma execução do histórico a nomear.

**Registo** num mosaico abre o que essa chave fez. Primeiro vêm as cópias que iniciou, cada uma com o seu estado e uma ligação a essa execução no registo de atividade do painel. Por baixo estão as chamadas, das mais recentes para as mais antigas, com a ferramenta e o que aconteceu à chamada. Uma recusa diz porquê: a chave só pode ler, a proteção de retenção travou a cópia, já havia outra cópia em curso, o item foi copiado por MCP há poucos minutos, ou a chave enviou demasiados pedidos. Um cancelamento liga à execução a que se referia.

O BombVault guarda as entradas de cada chave durante 30 dias no máximo: os 500 inícios, cancelamentos, recusas e erros mais recentes e, ao lado deles, as 200 leituras bem-sucedidas mais recentes. Assim, um assistente que consulta repetidamente uma cópia em curso não consegue empurrar o seu início para fora do registo. De cada chamada guarda a ferramenta, o resultado e a execução indicada por um cancelamento. Nunca guarda o que o assistente enviou, nem a chave ou a sua impressão digital. O pacote de diagnóstico só conta as entradas, e uma exportação das definições deixa-as de fora.

## Ligar um cliente {#clients}

O cartão mostra excertos prontos para o endereço com que o abriu: escolha o cliente e copie o excerto. O resto desta secção explica o que os excertos fazem e dá as formas que o cartão não mostra.

### Claude Code {#claude-code}

Execute uma vez num terminal o comando do cartão. Com um certificado em que o seu computador confia, fica assim:

```bash
claude mcp add --transport http bombvault --scope user https://bombvault.example.com/mcp --header "Authorization: Bearer <your key>"
```

Verifique a ligação com `/mcp` dentro do Claude Code. `--scope user` guarda a chave na sua configuração de utilizador e não num ficheiro de projeto.

O comando contém a chave, e a sua shell pode guardá-lo no histórico. Para o evitar, coloque um `.mcp.json` na pasta do projeto e guarde a chave numa variável de ambiente. O Claude Code substitui `${BOMBVAULT_MCP_KEY}` ao ler o ficheiro:

```json
{
  "mcpServers": {
    "bombvault": {
      "type": "http",
      "url": "https://bombvault.example.com/mcp",
      "headers": {
        "Authorization": "Bearer ${BOMBVAULT_MCP_KEY}"
      }
    }
  }
}
```

Defina `BOMBVAULT_MCP_KEY` onde o Claude Code arranca, por exemplo no perfil da sua shell, editado num editor de texto em vez de escrito na linha de comandos. Nunca faça commit de um `.mcp.json` com a chave escrita lá dentro.

Com o certificado próprio do BombVault (ver [TLS e certificados](#tls)), o comando do cartão corre em vez disso o `mcp-remote` e indica ao Node.js o certificado descarregado:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -e "BOMBVAULT_MCP_KEY=<your key>" -- npx -y mcp-remote https://192.168.1.10:3443/mcp --header 'X-API-Key:${BOMBVAULT_MCP_KEY}'
```

As plicas impedem a shell de expandir a variável; é o `mcp-remote` que o faz. A mesma forma funciona em `.mcp.json`: use a entrada do Claude Desktop abaixo e retire `BOMBVAULT_MCP_KEY` do seu `env`, assim a chave vem do seu ambiente.

### Claude Desktop {#claude-desktop}

O Claude Desktop chega ao BombVault através do `mcp-remote`, que precisa de Node.js nesse computador. Abra o ficheiro de configuração no Claude Desktop em **Settings, Developer, Edit Config**. Fica em `%APPDATA%\Claude\claude_desktop_config.json` no Windows e em `~/Library/Application Support/Claude/claude_desktop_config.json` no macOS. Acrescente a entrada do cartão dentro de `"mcpServers"`, ao lado dos servidores que já lá estejam, e reinicie o Claude Desktop:

```json
{
  "mcpServers": {
    "bombvault": {
      "command": "npx",
      "args": ["-y", "mcp-remote", "https://192.168.1.10:3443/mcp", "--header", "X-API-Key:${BOMBVAULT_MCP_KEY}"],
      "env": {
        "BOMBVAULT_MCP_KEY": "<your key>",
        "NODE_EXTRA_CA_CERTS": "<path of the downloaded bombvault-cert.pem>"
      }
    }
  }
}
```

- `NODE_EXTRA_CA_CERTS` só está lá para o certificado próprio do BombVault. Atrás de um certificado em que o seu computador já confia, retire-o.
- `--allow-http` só é acrescentado para um endereço `http://` simples.
- O cabeçalho escreve-se `X-API-Key:${BOMBVAULT_MCP_KEY}`, sem espaço depois dos dois pontos e com a chave em `env`. Em alguns sistemas o `mcp-remote` corta um valor de `--header` no primeiro espaço, e uma chave escrita depois de um espaço perder-se-ia.

### Conectores personalizados nas definições do Claude {#custom-connectors}

Os conectores adicionados nas próprias definições do Claude (em claude.ai e na lista de conectores do Claude Desktop) ainda não são suportados. Esses conectores são contactados a partir da nuvem da Anthropic, por isso precisam de um endereço HTTPS público, e iniciam sessão por OAuth. Não conseguem enviar uma chave fixa, e o BombVault só oferece chaves fixas, sem início de sessão OAuth. Pôr o BombVault na Internet por causa deles não ajudaria. Use o Claude Code, ou o Claude Desktop através do `mcp-remote` como acima.

### Outros clientes {#other-clients}

Serve qualquer cliente que fale Streamable HTTP:

- URL: o endereço da interface web mais `/mcp`, por exemplo `https://192.168.1.10:3443/mcp`.
- A chave em `Authorization: Bearer <key>` ou em `X-API-Key: <key>`. Se forem enviados os dois, têm de levar a mesma chave.
- `POST` com `Content-Type: application/json` e `Accept: application/json, text/event-stream`.
- Uma mensagem JSON-RPC por pedido; os lotes (batches) são recusados.
- Versões do protocolo 2026-07-28, 2025-11-25, 2025-06-18 e 2025-03-26.

## TLS e certificados {#tls}

O BombVault serve HTTPS com um certificado emitido por ele próprio, e de início esse certificado só nomeia `localhost`, `127.0.0.1` e `::1`. O Claude Code e o `mcp-remote` recusam-no num endereço da rede local. As saídas, pela ordem que serve à maioria das instalações Unraid:

1. **Acrescentar o endereço no cartão MCP.** Se abrir o cartão por HTTPS num endereço que o certificado não nomeia, ele avisa e oferece **Acrescentar este endereço ao certificado**. O BombVault emite de novo o certificado com esse endereço (o navegador avisa mais uma vez, como da primeira). Depois clique em **Descarregar certificado**; os excertos definem `NODE_EXTRA_CA_CERTS` para o ficheiro descarregado, e o cliente passa a confiar exatamente nesse certificado.
2. **Um proxy inverso com um certificado de confiança** (Nginx Proxy Manager, SWAG, Caddy, Traefik). O cliente vê então o certificado do proxy e não precisa de mais nada, e o cartão não avisa sobre o do BombVault.
3. **Tailscale.** `tailscale serve` à frente do contentor, ou a integração Tailscale do Unraid, dá-lhe um nome `ts.net` com um certificado de confiança.
4. **`HTTP_ONLY=true`**, só atrás de um proxy que termine o TLS ou numa rede em que confie totalmente. Passa toda a interface web para HTTP simples, exige uma alteração nas definições do contentor e envia a chave sem cifra.

Nunca defina `NODE_TLS_REJECT_UNAUTHORIZED=0`. Isso desliga a verificação de certificados para tudo aquilo com que esse processo Node.js fala.

Um proxy inverso tem de deixar passar o cabeçalho `Authorization` (ou `X-API-Key`), o que os proxies fazem a menos que lhes digam o contrário, e não pode reter em buffer nem reescrever `/mcp`. Um bloco location para Nginx ou Nginx Proxy Manager que também verifica o certificado do BombVault:

```nginx
location /mcp {
    proxy_pass https://192.168.1.10:3443;
    proxy_ssl_verify on;
    proxy_ssl_trusted_certificate /data/bombvault-cert.pem;
    proxy_ssl_name localhost;
    proxy_http_version 1.1;
    proxy_buffering off;
    proxy_set_header Host $host;
}
```

Atrás de um proxy, cada pedido traz o endereço do proxy. Cinco chaves erradas vindas de um único cliente mal configurado bloqueiam então durante um minuto todos os clientes MCP atrás desse proxy. Indique o proxy em `TRUSTED_PROXY` (ver [Configuração](configuration.md)) para contar por cliente.

## Modelo de segurança {#security}

- Sem uma chave ativa, `/mcp` responde `404`.
- Nenhum endereço está isento. Pedidos vindos de `localhost`, do anfitrião Unraid, de um proxy inverso ou de `tailscale serve` precisam de uma chave como qualquer outro, também quando a interface web não tem palavra-passe de início de sessão.
- As chaves só são guardadas como impressão digital, mostradas uma vez, e podem ser renomeadas, substituídas e revogadas. Até 10 chaves ativas, cada uma com o seu interruptor **Permitir iniciar cópias**.
- Cada criação, substituição, mudança de permissão e revogação envia uma notificação pelos seus canais de notificação, com o endereço de onde veio, a não ser que as notificações estejam desligadas.
- 5 chaves erradas por minuto e por endereço, depois `429`. 120 pedidos por minuto e 12 cópias iniciadas por hora e por chave, mais a espera e a proteção da retenção descritas acima.
- Pedidos de uma página de navegador de outra origem são recusados.
- Enquanto não houver palavra-passe de início de sessão, não é possível criar chaves a partir de um nome de anfitrião que pareça público.
- Cada cópia que um assistente inicia, e as execuções de prune e de cópia externa que daí resultam, fica marcada "via MCP" com o nome da chave no registo de atividade, no painel de erros e na notificação da cópia.
- Cada chamada a uma ferramenta é escrita no registo do contentor com o id da chave e os seus últimos quatro caracteres (nunca o nome) e contada em `/metrics` (`bombvault_mcp_requests_total`, `bombvault_mcp_tool_calls_total`, `bombvault_mcp_active_keys`).
- Restaurar uma cópia da configuração revoga todas as chaves, porque a base de dados restaurada pode conter chaves que revogou depois de ela ter sido guardada. Crie chaves novas a seguir.
- Uma chave deixa de funcionar quando `APP_KEY` muda (uma reinstalação, ou um restauro noutro contentor). O cartão deteta isso e marca a chave, e **Substituir chave** volta a dar-lhe um segredo válido.
- Trate uma chave como uma palavra-passe. O Claude Code e o Claude Desktop guardam-na em texto simples na sua configuração. Num computador em que confie menos, prefira uma chave só de leitura.

## O que sai da máquina {#privacy}

Tudo o que um assistente lê vai para o fornecedor de IA por trás dele: nomes dos elementos, agendamentos, histórico de execuções com mensagens de erro, ids e horas dos pontos de restauro, nomes dos motores de base de dados e tamanhos dos dumps, atividade em curso, números de armazenamento, cobertura e estado. O BombVault retira caminhos do anfitrião, localizações dos repositórios, nomes de anfitrião, credenciais, comandos de hook e chaves antes de alguma coisa sair.

## Resolução de problemas {#troubleshooting}

| O que vê | O que significa |
|---|---|
| `404` | Não há chave ativa, ou o caminho está errado, como `/api/mcp`. O ponto de ligação é `/mcp`. |
| `401` | A chave falta, está mal escrita, revogada ou substituída. Pode ser que um proxy descarte o cabeçalho `Authorization` (experimente `X-API-Key`). Se o cartão marcar a chave como já não válida, `APP_KEY` mudou: substitua a chave. |
| `403` | O pedido veio de uma página de navegador de outra origem. Use um cliente de secretária ou de linha de comandos. |
| `405` em GET | Normal. O ponto de ligação só aceita `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | O cliente é demasiado antigo para Streamable HTTP. Atualize-o. |
| `400` "batch requests are not accepted" | O cliente envia lotes JSON-RPC. Envie uma mensagem por pedido. |
| `429` | Demasiadas chaves erradas deste endereço, ou mais de 120 pedidos por minuto com uma chave. Espere um minuto e verifique se o assistente está preso num ciclo. |
| Erros com "certificate", "self-signed" ou "unable to verify" | O cliente não confia no certificado do BombVault. Ver [TLS e certificados](#tls). |
| `busy` | Outra cópia ou uma tarefa de manutenção ocupa esse domínio. Tente de novo quando terminar. |
| `cooldown` | Este elemento, este domínio ou Backup Everything foi iniciado por MCP há menos de 15 minutos. |
| `retention_guard` | Mais uma cópia MCP deixaria só pontos de restauro vindos de MCP numa janela "manter os últimos N". A próxima cópia agendada abre espaço, ou inicie-a na interface web. |
| `rate_limited` | A chave gastou os seus 12 inícios desta hora. |
| `not_permitted` num início | A chave é só de leitura. Ligue **Permitir iniciar cópias** no cartão; não é preciso voltar a ligar. Num cancelamento significa que a execução não foi iniciada por esta chave. |
| `domain_off` | Esse tipo de cópia está desligado nas definições. |
| `not_found` | O BombVault não protege esse elemento. Acrescente-o primeiro na interface web; o MCP nunca cria configuração. |

Não defina a variável de ambiente `MCPGODEBUG` no contentor. Altera o comportamento da biblioteca MCP, e um valor mal formado para o BombVault no arranque antes de escrever uma única linha de registo.
