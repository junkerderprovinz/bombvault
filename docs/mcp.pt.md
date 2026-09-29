# Servidor MCP

O BombVault tem um servidor integrado para o Model Context Protocol (MCP), o protocolo com que assistentes de IA como o Claude Code e o Claude Desktop chegam a ferramentas externas. Através dele, um assistente pode ler como estão as tuas cópias e, se o permitires, iniciar uma cópia ou cancelar uma que ele próprio iniciou. Está desligado até criares uma chave ou ligares a [autenticação por OAuth](#oauth): até lá, o endpoint `/mcp` responde `404` a tudo.

## O que um assistente pode e não pode fazer {#tools}

| Ferramenta | O que faz | Tipo |
|---|---|---|
| `get_health` | Versão, nome da instância, se há uma cópia em curso e o que esta chave pode fazer | leitura |
| `get_status` | Estado de proteção por domínio: última cópia bem-sucedida, intervalo esperado, verificações e controlos externos, próximas execuções agendadas, cópias à espera de que a app fique inativa e, para os contentores, o teste de arranque mais recente | leitura |
| `get_coverage` | O que o BombVault protege e o que não protege, com o motivo de cada caso | leitura |
| `list_items` | Cada contentor, VM e conjunto de pastas protegido, a pen flash e a configuração da app, com o agendamento, o que uma cópia para, a última cópia e quanto demorou; os contentores de base de dados indicam também o último dump; os datasets ZFS também aparecem, com o resultado da sua última verificação; cada item traz a sua última verificação de restauro, e um contentor o seu último teste de arranque ou o motivo por que não pode ser testado; um contêiner recriado com outras configurações desde o último backup indica o que mudou | leitura |
| `list_runs` | Histórico de execuções, as mais recentes primeiro, filtrável por domínio, elemento, estado, tipo e data; um backup lento travado por uma única coisa nomeia-a | leitura |
| `list_restore_points` | Pontos de restauro de um elemento no seu repositório principal e, para um contentor, os seus dumps de base de dados; um dataset ZFS tem um ponto de restauro por cópia, com um snapshot de cada dataset abaixo dele | leitura |
| `get_activity` | O que está a correr agora, com fase e percentagem | leitura |
| `get_storage_stats` | Histórico de tamanho do repositório principal de um domínio e o seu crescimento semanal, com o espaço usado, livre e total no disco ou remoto de cada um dos seus repositórios | leitura |
| `get_size_breakdown` | Que pastas e arquivos ocupam espaço no backup mais recente de um contêiner, uma VM ou um conjunto de pastas, e quanto disso o último backup acrescentou | leitura |
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
- **Proteção da retenção.** Quando um domínio mantém um número fixo de pontos de restauro (só "manter os últimos N", sem regra diária, semanal ou mensal, localmente ou num destino externo), cada cópia nova faz sair a mais antiga. O BombVault recusa então um início MCP de um elemento cujas N-1 cópias bem-sucedidas mais recentes foram todas iniciadas por MCP. Assim fica sempre no conjunto mantido pelo menos um ponto de restauro criado pelo agendamento ou por si. Com "manter o último" (N = 1), um assistente não consegue fazer cópia desse elemento. A próxima cópia agendada volta a abrir espaço. Uma regra anual sozinha conta como "manter o último" (N = 1), porque mantém um único ponto de restauro do ano corrente.

Um início de domínio ou de Backup Everything deixa de fora os elementos que um limite retém e indica-os na resposta. A interface web e o agendamento não são afetados por nada disto. A quota horária vive em memória, por isso um reinício do BombVault repõe-na a zero.

Os inícios pela [API](api.md#errors) e a partir do [Home Assistant](api.md#home-assistant) contam para os mesmos limites por item que os inícios por MCP, e para a proteção da retenção.

## Ativar {#switch-on}

1. Abre **Definições, Sistema, Servidor MCP** e clica no botão do teu cliente. Um cliente que não está na lista liga-se através de **Outro cliente**.
2. Em **Chave**, deixa **Chave nova** e o nome proposto, o do cliente, ou escreve um que diga onde a chave é usada, por exemplo «Claude Code no portátil». Uma chave por cliente permite revogar uma sem mexer nas outras. **Chave existente** dá ao cliente uma chave que criaste antes.
3. Liga **Permitir iniciar cópias** para uma chave que deva poder iniciar cópias; sem isso só pode ler. Podes mudar isto mais tarde no mosaico da chave, e a alteração vale a partir do pedido seguinte do assistente, sem nova ligação.
4. Clica em **Criar chave**. A chave é mostrada uma vez. O BombVault guarda apenas uma impressão digital dela e não a pode mostrar outra vez, por isso copia-a agora. Se fechares o diálogo antes de o cliente ter usado a chave, o cartão continua a mostrá-la até confirmares que a copiaste.

Sem palavra-passe de início de sessão, a própria interface web está aberta a toda a sua rede, e quem a conseguir abrir também pode criar uma chave. O cartão avisa disso. Se abrir o BombVault com um nome que parece público (por exemplo `bombvault.example.com` atrás de um proxy inverso) e não houver palavra-passe de início de sessão, a partir desse endereço não é possível criar nem substituir chaves, para que nenhuma página da Internet consiga levar o seu navegador a criar uma. Defina uma palavra-passe de início de sessão, ou abra o BombVault pelo endereço IP ou por um nome local como `tower` ou `tower.local`.

## As suas chaves e o registo delas {#keys}

Cada chave tem o seu próprio mosaico no cartão. Mostra o nome da chave, se pode iniciar cópias ou só ler, os quatro últimos caracteres da chave, quando foi criada ou substituída pela última vez, quando um cliente a usou pela última vez e quantas chamadas fez hoje. No mosaico muda o nome da chave, altera a permissão, substitui-a ou revoga-a. Uma chave revogada passa para a lista de chaves revogadas, onde a pode apagar de vez quando nenhuma execução do histórico a nomear.

Ao lado do nome, o mosaico mostra o logótipo do cliente para o qual a chave foi criada. Uma chave criada através de **Outro cliente**, ou antes de o cartão listar clientes, mostra uma chave no seu lugar.

**Registo** num mosaico abre o que essa chave fez. Primeiro vêm as cópias que iniciou, cada uma com o seu estado e uma ligação a essa execução no registo de atividade do painel. Por baixo estão as chamadas, das mais recentes para as mais antigas, com a ferramenta e o que aconteceu à chamada. Uma recusa diz porquê: a chave só pode ler, a proteção de retenção travou a cópia, já havia outra cópia em curso, uma cópia do item foi iniciada fora da interface web há poucos minutos, ou a chave enviou demasiados pedidos. Um cancelamento liga à execução a que se referia.

O BombVault guarda as entradas de cada chave durante 30 dias no máximo: os 500 inícios e cancelamentos bem-sucedidos mais recentes e, ao lado deles, as 200 outras chamadas mais recentes (leituras, recusas e erros). Assim, um assistente que consulta repetidamente uma cópia em curso, ou que repete uma chamada recusada, não consegue empurrar o seu início para fora do registo. De cada chamada guarda a ferramenta, o resultado e a execução indicada por um cancelamento. Nunca guarda o que o assistente enviou, nem a chave ou a sua impressão digital. O pacote de diagnóstico só conta as entradas, e uma exportação das definições deixa-as de fora.

## Ligar um cliente {#clients}

Cada cliente tem um botão no cartão, em **Neste computador** ou em **Na nuvem**. O botão abre um diálogo em três passos: a chave; a configuração para esse cliente, com o endereço com que abriste o cartão, um botão para a copiar, o sítio onde a configuração fica e, com o certificado próprio do BombVault, o que o cliente precisa para confiar nele; e a espera pela primeira chamada do cliente. O diálogo acompanha a última utilização da chave e fica verde quando essa chamada chega.

O diálogo mantém a chave fora de qualquer linha de comandos. Quando o cliente a consegue ler de uma variável de ambiente (`BOMBVAULT_MCP_KEY`), de um pedido oculto ou de um ficheiro seu, a configuração apenas a nomeia. Quando o cliente não tem essa forma, a chave fica no ficheiro de configuração ou nas definições dele, e o diálogo diz isso. Quando a documentação de um cliente não diz como trata um certificado que não conhece, o diálogo escreve esse passo como o que fazer se o cliente recusar o certificado do BombVault.

| Cliente | Configuração | De onde vem a chave |
|---|---|---|
| AnythingLLM | ficheiro de configuração | o ficheiro de configuração |
| Antigravity | ficheiro de configuração | variável de ambiente |
| Claude Code | comando | ficheiro da chave |
| Claude Desktop | ficheiro de configuração | ficheiro da chave |
| Cline | ficheiro de configuração | o ficheiro de configuração |
| Codex CLI | ficheiro de configuração | variável de ambiente |
| Continue | ficheiro de configuração | `~/.continue/.env` |
| Copilot CLI | ficheiro de configuração | o ficheiro de configuração |
| Cursor | ficheiro de configuração | variável de ambiente |
| Gemini CLI | ficheiro de configuração | variável de ambiente |
| GitHub Copilot (VS Code) | ficheiro de configuração | pedido oculto |
| Goose | ficheiro de configuração | variável de ambiente |
| Jan | formulário na app | as definições da app |
| JetBrains (AI Assistant, Junie) | ficheiro de configuração | o ficheiro de configuração |
| Kimi Code | ficheiro de configuração | o ficheiro de configuração |
| LM Studio | ficheiro de configuração | o ficheiro de configuração |
| Mistral Vibe | ficheiro de configuração | variável de ambiente |
| Msty | formulário na app | as definições da app |
| n8n | formulário na app | as credenciais do n8n |
| Open WebUI | formulário na app | as definições da app |
| opencode | ficheiro de configuração | variável de ambiente |
| Perplexity (Mac) | formulário na app | ficheiro da chave |
| Qwen Code | ficheiro de configuração | variável de ambiente |
| Roo Code | ficheiro de configuração | variável de ambiente |
| Visual Studio | ficheiro de configuração | o ficheiro de configuração |
| Warp | ficheiro de configuração | o ficheiro de configuração |
| Windsurf | ficheiro de configuração | variável de ambiente |
| Zed | ficheiro de configuração | o ficheiro de configuração |
| Grok | formulário, na nuvem | os servidores do fornecedor |
| Le Chat | formulário, na nuvem | os servidores do fornecedor |
| ChatGPT | autenticação por OAuth, na nuvem | um token de acesso, ver [abaixo](#oauth) |
| Claude (claude.ai) | autenticação por OAuth, na nuvem | um token de acesso, ver [abaixo](#oauth) |

As secções abaixo explicam com mais pormenor a configuração do Claude Code e do Claude Desktop e indicam o que qualquer outro cliente precisa.

### Claude Code {#claude-code}

O Claude Code chega ao BombVault através do `mcp-remote`, que precisa de Node.js nesse computador. Primeiro guarde a chave num ficheiro de texto à parte, numa única linha:

```text
X-API-Key: <your key>
```

Depois execute uma vez num terminal o comando do cartão, com o caminho desse ficheiro. Atrás de um certificado em que o seu computador confia, fica assim:

```bash
claude mcp add bombvault --scope user -- npx -y mcp-remote@latest https://bombvault.example.com/mcp --header-file "<path of the file with your key>"
```

Com o certificado próprio do BombVault (ver [TLS e certificados](#tls)), o comando indica também ao Node.js o certificado descarregado:

```bash
claude mcp add bombvault --scope user -e "NODE_EXTRA_CA_CERTS=<path of the downloaded bombvault-cert.pem>" -- npx -y mcp-remote@latest https://192.168.1.10:3443/mcp --header-file "<path of the file with your key>"
```

Verifique a ligação com `/mcp` dentro do Claude Code. `--scope user` torna o BombVault disponível em todos os seus projetos. O Claude Code guarda só o caminho do ficheiro da chave, por isso a chave não aparece no comando nem no histórico da shell, nem na lista de processos. Guarde o ficheiro onde mais ninguém o possa ler, e fora de qualquer pasta de que faça commit. `@latest` faz o `npx` ir buscar um `mcp-remote` atual; caso contrário seria usado um mais antigo instalado globalmente, que não conhece `--header-file`.

Não escreva `${BOMBVAULT_MCP_KEY}` nos argumentos do `mcp-remote` para o Claude Code. O Claude Code substitui essa referência a partir do seu próprio ambiente antes de iniciar o `mcp-remote`, por isso a chave acaba na linha de comandos desse processo, onde outros programas e utilizadores do computador a podem ler.

Sem Node.js, e só atrás de um certificado em que o seu computador confia, o Claude Code consegue ligar-se sozinho. Coloque um `.mcp.json` na pasta do projeto:

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

Defina `BOMBVAULT_MCP_KEY` onde o Claude Code arranca, por exemplo em `"env"` no `~/.claude/settings.json` ou no perfil da sua shell, editado num editor de texto em vez de escrito na linha de comandos. Aqui a referência é segura, porque o Claude Code não inicia um segundo processo que a leve consigo. O certificado próprio do BombVault não funciona desta forma: a ligação que o próprio Claude Code abre recusa-o, mesmo com `NODE_EXTRA_CA_CERTS` definido. Nunca faça commit de um `.mcp.json` com a chave escrita lá dentro.

### Claude Desktop {#claude-desktop}

O Claude Desktop chega ao BombVault através do `mcp-remote`, que precisa de Node.js nesse computador. Guarde primeiro a chave num ficheiro de texto próprio, numa única linha, como descrito para o [Claude Code](#claude-code). Abra o ficheiro de configuração no Claude Desktop em **Settings, Developer, Edit Config**. Fica em `%APPDATA%\Claude\claude_desktop_config.json` no Windows e em `~/Library/Application Support/Claude/claude_desktop_config.json` no macOS. Acrescente a entrada do cartão dentro de `"mcpServers"`, ao lado dos servidores que já lá estejam, e reinicie o Claude Desktop:

```json
{
  "mcpServers": {
    "bombvault": {
      "command": "npx",
      "args": ["-y", "mcp-remote@latest", "https://192.168.1.10:3443/mcp", "--header-file", "<path of the file with your key>"],
      "env": {
        "NODE_EXTRA_CA_CERTS": "<path of the downloaded bombvault-cert.pem>"
      }
    }
  }
}
```

- `NODE_EXTRA_CA_CERTS` só está lá para o certificado próprio do BombVault. Atrás de um certificado em que o seu computador já confia, retire-o.
- `--allow-http` só é acrescentado para um endereço `http://` simples.
- No Windows, escreva os caminhos com barras normais, por exemplo `C:/Users/sam/bombvault-key.txt`, porque uma barra invertida sozinha não é JSON válido. Mantenha o caminho do ficheiro da chave sem espaços: o Claude Desktop no Windows passa ao `npx` um caminho com um espaço em dois pedaços.
- A configuração só indica o ficheiro da chave, por isso a chave não aparece nela nem na lista de processos. Guarde o ficheiro onde mais ninguém o possa ler.

### Clientes na nuvem {#cloud-clients}

O ChatGPT, o Claude em claude.ai, o Grok e o Le Chat chamam o BombVault a partir dos servidores do seu fornecedor, por isso o BombVault tem de estar acessível a partir da Internet com um certificado de confiança pública, por exemplo atrás de um proxy inverso; o Le Chat recusa certificados autoassinados. Um início de sessão no proxy pode proteger a interface web, mas `/mcp` tem de chegar ao BombVault sem ele: estes serviços não conseguem iniciar sessão num proxy, e o BombVault verifica ele próprio a chave ou o token. O Grok e o Le Chat enviam uma chave fixa, e os seus botões configuram-nos como os outros. O ChatGPT, e o Claude em claude.ai na maioria das organizações, só se ligam através de uma autenticação por OAuth, descrita a seguir.

### Autenticação por OAuth {#oauth}

Para um cliente que não aceita uma chave, o BombVault é o seu próprio servidor de autorização OAuth. O cliente regista-se sozinho, envia-te para uma página do BombVault, e aí entras com a tua palavra-passe de acesso (e o segundo fator, se o configuraste) e autoriza-lo. O cliente recebe depois um token que só serve para o endpoint MCP deste BombVault, e renova-o sozinho.

1. Define uma palavra-passe de acesso em **Definições, Sistema**. Sem ela o BombVault não oferece nenhuma autenticação, porque não haveria ninguém a quem pedir consentimento.
2. Torna o BombVault acessível a partir da Internet por https com um certificado em que os browsers confiem, normalmente através de um proxy inverso. O cliente chama `/mcp`, `/oauth/` e `/.well-known/` a partir dos seus próprios servidores, por isso um proxy com início de sessão próprio tem de deixar passar esses três caminhos até ao BombVault. A página de consentimento em `/oauth/authorize` abre no teu próprio browser e pode ficar atrás do início de sessão do proxy. Indica também o proxy em `TRUSTED_PROXY` (ver [Configuração](configuration.md)). O BombVault limita os registos de clientes por endereço e, sem isso, todos os clientes parecem vir do proxy.
3. No cartão MCP, liga **Autenticação por OAuth** e introduz o **Endereço público**: o endereço https sem caminho, por exemplo `https://backup.example.com`. Cada token fica ligado a este endereço, por isso após uma alteração cada cliente tem de se autenticar de novo.
4. Clica no botão do ChatGPT ou do Claude. O diálogo mostra o **URL do conector**, ou seja, o endereço público seguido de `/mcp`, e onde entra nesse cliente. No ChatGPT, liga o modo de programador em **Definições, Apps e conectores, Definições avançadas**, escolhe **Criar**, cola o URL do conector como URL do servidor MCP e escolhe OAuth como autenticação. Em claude.ai, abre **Definições, Conectores, Adicionar conector personalizado**, cola o URL do conector, deixa vazios o ID de cliente e o segredo OAuth e escolhe **Ligar**.
5. O cliente abre a página de consentimento. Mostra quem pede, para onde a tua resposta te leva de volta e o interruptor **Permitir iniciar cópias**, que começa desligado. Escolhe **Permitir** ou **Recusar**.

Cada cliente autenticado recebe um mosaico ao lado das chaves, com o seu logótipo, o seu registo, **Revogar** e **Permitir iniciar cópias**, e os mesmos limites de uma chave. A revogação tem efeito imediato. Quando o mesmo cliente se autentica de novo, a nova autorização substitui a anterior, e uma autorização que ninguém usou durante 30 dias expira. Podem estar autenticados até 10 clientes ao mesmo tempo, além das 10 chaves.

A página de consentimento só aceita um pedido de um cliente registado que indique exatamente um dos seus endereços de regresso registados: https, ou um endereço de loopback em qualquer porta para um cliente no teu próprio computador. Só é aceite o fluxo de código de autorização com PKCE (S256), e a tua resposta fica ligada à tua sessão, por isso nenhum outro site a pode enviar por ti. Os tokens de acesso duram uma hora. Um token de atualização é substituído a cada utilização, e se voltar a aparecer depois, o BombVault revoga a autorização, porque outra pessoa tem uma cópia. Um cliente que repete a sua última atualização em menos de 30 segundos, porque a resposta nunca lhe chegou, recebe em vez disso tokens novos. O BombVault não descarrega metadados de clientes da Internet, por isso os clientes registam-se através do registo dinâmico de clientes.

### Outros clientes {#other-clients}

Serve qualquer cliente que fale Streamable HTTP:

- URL: o endereço da interface web mais `/mcp`, por exemplo `https://192.168.1.10:3443/mcp`.
- A chave em `Authorization: Bearer <key>` ou em `X-API-Key: <key>`. Se forem enviados os dois, têm de levar a mesma chave.
- `POST` com `Content-Type: application/json` e `Accept: application/json, text/event-stream`.
- Uma mensagem JSON-RPC por pedido; os lotes (batches) são recusados.
- Versões do protocolo 2026-07-28, 2025-11-25, 2025-06-18 e 2025-03-26.

## TLS e certificados {#tls}

O BombVault serve HTTPS com um certificado emitido por ele próprio, e de início esse certificado só nomeia `localhost`, `127.0.0.1` e `::1`. O Claude Code e o `mcp-remote` recusam-no num endereço da rede local. As saídas, pela ordem que serve à maioria das instalações Unraid:

1. **Acrescentar o endereço no cartão MCP.** Se abrir o cartão por HTTPS num endereço que o certificado não nomeia, ele avisa e oferece **Acrescentar este endereço ao certificado**. O BombVault emite de novo o certificado com esse endereço (o navegador avisa mais uma vez, como da primeira). Depois clique em **Descarregar certificado**; os excertos definem `NODE_EXTRA_CA_CERTS` para o ficheiro descarregado, e o cliente passa a confiar exatamente nesse certificado. Isto também significa que qualquer cliente configurado com um ficheiro descarregado antes deixa de se ligar assim que o certificado é emitido de novo, neste computador e em todos os outros, até receber o ficheiro novo.
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

- Sem uma chave ativa e com a autenticação por OAuth desligada, `/mcp` responde `404`.
- A autenticação por OAuth só é oferecida enquanto houver uma palavra-passe de acesso. Tokens, códigos e segredos de cliente são guardados apenas como impressão digital, e um token só serve para o endereço para o qual foi emitido.
- Um cliente pode registar-se no máximo 10 vezes por hora a partir do mesmo endereço, e o BombVault guarda no máximo 100 clientes registados com que ninguém se autenticou, cada um durante um dia. Códigos e tokens de atualização errados contam para o mesmo bloqueio que as chaves erradas.
- As autorizações comportam-se como as chaves ao restaurar uma cópia da configuração ou ao mudar o `APP_KEY`: após um restauro, cada cliente tem de se autenticar de novo.
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
- Trata uma chave como uma palavra-passe. Um cliente que não consegue ler a chave de uma variável de ambiente, de um pedido ou de um ficheiro da chave guarda-a em texto simples na sua configuração ou nas suas definições, e o diálogo dele diz isso. Num computador em que confias menos, prefere uma chave só de leitura.

## O que sai da máquina {#privacy}

Tudo o que um assistente lê vai para o fornecedor de IA por trás dele: nomes dos elementos, agendamentos, histórico de execuções com mensagens de erro, ids e horas dos pontos de restauro, nomes dos motores de base de dados e tamanhos dos dumps, atividade em curso, números de armazenamento, cobertura e estado. O BombVault retira caminhos do anfitrião, localizações dos repositórios, nomes de anfitrião, credenciais, comandos de hook e chaves antes de alguma coisa sair.

## Resolução de problemas {#troubleshooting}

| O que vê | O que significa |
|---|---|
| `404` | Não há nenhuma chave ativa e a autenticação por OAuth está desligada, ou o caminho está errado, como `/api/mcp`. O endpoint é `/mcp`. |
| `401` | A chave falta, está mal escrita, revogada ou substituída. Pode ser que um proxy descarte o cabeçalho `Authorization` (experimente `X-API-Key`). Se o cartão marcar a chave como já não válida, `APP_KEY` mudou: substitua a chave. |
| `403` | O pedido veio de uma página de navegador de outra origem. Use um cliente de secretária ou de linha de comandos. |
| `405` em GET | Normal. O ponto de ligação só aceita `POST`. |
| `400` "Accept must contain both 'application/json' and 'text/event-stream'" | O cliente é demasiado antigo para Streamable HTTP. Atualize-o. |
| `400` "batch requests are not accepted" | O cliente envia lotes JSON-RPC. Envie uma mensagem por pedido. |
| `429` | Demasiadas chaves erradas deste endereço, ou mais de 120 pedidos por minuto com uma chave. Espere um minuto e verifique se o assistente está preso num ciclo. |
| Erros com "certificate", "self-signed" ou "unable to verify" | O cliente não confia no certificado do BombVault. Ver [TLS e certificados](#tls). |
| `busy` | Outra cópia ou uma tarefa de manutenção ocupa esse domínio. Tente de novo quando terminar. |
| `cooldown` | Este elemento, este domínio ou Backup Everything foi iniciado fora da interface web há menos de 15 minutos. |
| `retention_guard` | Mais uma cópia MCP deixaria só pontos de restauro vindos de MCP numa janela "manter os últimos N", ou o elemento já recebeu 4 cópias por MCP nas últimas 24 horas, contando as que falharam e as canceladas. No primeiro caso a próxima cópia agendada abre espaço; no segundo o elemento fica livre outra vez 24 horas depois da mais antiga dessas cópias. Em qualquer dos casos pode iniciá-la na interface web. |
| `rate_limited` | A chave gastou os seus 12 inícios desta hora. |
| `not_permitted` num início | A chave é só de leitura. Ligue **Permitir iniciar cópias** no cartão; não é preciso voltar a ligar. Num cancelamento significa que a execução não foi iniciada por esta chave. |
| `domain_off` | Esse tipo de cópia está desligado nas definições. |
| `not_found` | O BombVault não protege esse elemento. Acrescente-o primeiro na interface web; o MCP nunca cria configuração. |
| O cliente não encontra o servidor de autorização | A autenticação por OAuth está desligada, não há palavra-passe de acesso, ou o proxy não deixa passar `/.well-known/` até ao BombVault. |
| A página de consentimento diz que o endereço de regresso não está registado | O cliente enviou um endereço de regresso que não registou. Remove o conector no cliente e volta a adicioná-lo. |
| Um cliente autenticado recebe `401` | A sua autorização foi revogada, expirou após 30 dias sem uso, ou o endereço público mudou. O cliente autentica-se de novo. |

Não defina a variável de ambiente `MCPGODEBUG` no contentor. Altera o comportamento da biblioteca MCP, e um valor mal formado para o BombVault no arranque antes de escrever uma única linha de registo.
