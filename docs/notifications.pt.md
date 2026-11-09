# Notificações

O BombVault pode avisar-te quando um backup falha, quando tem êxito, quando há um em atraso, e uma vez por semana como correram as coisas. Tudo se define em **Definições, Notificações**: o que o BombVault envia além dos backups individuais no cartão **Notificações**, e para onde vão as mensagens no cartão **Canais**. Cada canal decide por si quando se manifesta, e uma verificação do Healthchecks também é um canal.

![Definições, Notificações com o canal Gotify preenchido.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Definições, Notificações com o canal Gotify preenchido.](assets/screenshots/notifications/gotify-light.png#only-light)

## O que envia uma mensagem {#what}

Os interruptores do cartão **Notificações** valem para todos os canais:

- **Resumir execuções agendadas**: uma mensagem por execução agendada, por exemplo *42 de 45 com êxito*, em vez de uma por container ou VM. Um backup que inicias à mão continua a enviar a sua própria mensagem. Um convidado congelado, uma retenção falhada ou um orçamento de crescimento ultrapassado seguem logo, mesmo com os resumos ligados.
- **Notificar ao atualizar container**: uma mensagem por cada container que **Atualizar contêineres após um backup bem-sucedido** passou para uma imagem mais recente, para poderes ver se continua a funcionar.
- **Comunicar backups em atraso**: uma vez por dia, às 09:00, o BombVault verifica se algum backup ligado está em atraso, ou seja, mais velho do que o dobro do seu agendamento, e envia uma mensagem por caso, com o nome da entrada que ficou para trás. O backup bem-sucedido seguinte limpa-o.
- **Resumo semanal**: um resumo por semana com o número de execuções, os novos dados de backup, as secções em atraso, se as cópias externas estão atuais e as principais falhas. O seu plano é um botão como todos os outros planos, e **Desativado** desliga-o. Vê [Agendamentos](schedules.md#plans).

Se um canal também recebe os sucessos define-se no próprio canal, em **Notificar**. Que anomalias enviam uma mensagem define-se em **Definições, Verificação**, vê [Anomalias](anomalies.md). Um backup adiado envia uma mensagem quando começa a esperar e outra quando a espera termina, não uma por tentativa.

## Canais {#channels}

**Adicionar canal**, no fundo do cartão **Canais**, oferece **Notificações do Unraid**, **ntfy**, **Gotify**, **Discord**, **Slack**, **Matrix**, **E-mail (SMTP)**, **Apprise**, **Webhook** e **Healthchecks**. Junta quantos quiseres, também dois do mesmo tipo, por exemplo um tópico ntfy para as falhas e outro para tudo. Um canal novo começa desligado, para ficar guardado antes de os seus campos estarem preenchidos.

Cada canal tem:

- o seu próprio interruptor, que o desliga sem perder os seus campos.
- **Notificar**: **Nunca**, **Apenas em caso de falha** ou **Em sucesso e falha**. Um canal novo começa em **Apenas em caso de falha**. Um backup que teve êxito mas deixou algo de fora também chega a um canal só de falhas.
- **Nome**, que é opcional e aparece no lugar do tipo, útil quando tens dois do mesmo tipo.
- **Enviar teste**, que envia uma mensagem de teste por este canal com os valores tal como estão no formulário, guardados ou não. A linha por baixo do canal diz se o serviço a aceitou, e se não, porquê.
- **Remover canal**, o caixote do lixo ao lado.

Cada canal envia por si, com o seu próprio limite de tempo, por isso um host que nunca responde não atrasa os outros. Quando um serviço falha de uma forma que outra tentativa poderia resolver, o canal tenta até três vezes. Um alerta de atraso, de orçamento ou de anomalia que nenhum canal aceitou é tentado outra vez mais tarde, e o mesmo vale para o resumo semanal.

| Canal | Campos | Notas |
|---|---|---|
| **Notificações do Unraid** | nenhum | Envia para o sistema de notificações do próprio Unraid, que pode reencaminhar para o Pushover, e-mail ou Discord. Corre pela ligação SSH de **Definições, Integrações, SSH do anfitrião**, por isso essa chave tem de estar autorizada. No TrueNAS e noutros hosts fica em silêncio, e o canal diz porquê. |
| **ntfy** | **URL do tópico**, **Token de acesso (opcional)** | O endereço termina no teu tópico. Só precisas de um token para um tópico protegido. |
| **Gotify** | **URL do servidor**, **Token da aplicação**, **Prioridade em caso de falha**, **Prioridade em caso de sucesso** | As prioridades vão de 0 a 10 e começam em 8 para falhas e 5 para sucessos. |
| **Discord** | **URL do Webhook** | O endereço de um webhook das definições do canal do Discord. |
| **Slack** | **URL do Webhook** | O endereço de um webhook de entrada da tua aplicação Slack. |
| **Matrix** | **URL do Homeserver**, **Token de acesso**, **ID da sala** | A conta por trás do token tem de estar na sala. |
| **E-mail (SMTP)** | **Host SMTP**, **Porta**, **Encriptação**, **Nome de utilizador**, **Palavra-passe**, **Endereço de origem**, **Endereço de destino** | A **Encriptação** é **STARTTLS**, **TLS (implicit)** ou **None**. |
| **Apprise** | **URL da API Apprise**, **Etiquetas (opcional)**, **Endereços Apprise (opcional)** | Aponta-o para o endpoint `/notify/<key>` do teu container Apprise API para chegar aos seus mais de 100 serviços, e as etiquetas escolhem um subconjunto dos destinos dessa chave. Ou indica endereços Apprise, um por linha, por exemplo `tgram://` para o Telegram ou `pover://` para o Pushover, e termina o URL da API em `/notify`, sem chave. |
| **Webhook** | **URL do Webhook**, **Método**, **Cabeçalhos**, **Corpo** | Um pedido que montas tu, vê [Webhook](#webhook). |
| **Healthchecks** | **URL de ping** | Vê [Healthchecks](#healthchecks). |

Os segredos, como os tokens, a palavra-passe do SMTP e os valores dos cabeçalhos do webhook, ficam guardados encriptados, e o servidor nunca os devolve à página. Por baixo de um segredo guardado o campo diz **Guardado. Deixe o campo vazio para o manter.** Quando mudas para onde um canal envia, o BombVault não envia o segredo antigo para o novo sítio: o campo diz **O novo endereço é guardado assim que introduzir isto de novo.**, e o novo endereço fica guardado quando o escreves.

As definições de uma versão anterior à lista de canais são transferidas no primeiro arranque, e nada se perde. Cada canal que tinhas preenchido passa a ser uma entrada própria, com a escolha de **Notificar** que tinhas. Um webhook no formato Discord, Slack, Gotify ou ntfy passa a ser um canal desse tipo, e um webhook em JSON genérico continua a enviar `title`, `message` e `ok` pelo seu **Corpo**. O interruptor do Unraid passa a ser um canal do Unraid, e o URL do Healthchecks com as suas exceções um canal do Healthchecks.

### Webhook {#webhook}

Um canal **Webhook** envia um pedido HTTP para qualquer endereço. **Método** é GET, POST ou PUT. **Adicionar cabeçalho** junta uma linha em **Cabeçalhos**, e os valores dos cabeçalhos são guardados como segredos, por isso um cabeçalho `Authorization` fica seguro aí. O **Corpo** é enviado tal como o escreves, com estes marcadores preenchidos:

| Marcador | Passa a ser |
|---|---|
| `%%title%%` | o título da mensagem |
| `%%body%%` | a própria mensagem |
| `%%ok%%` | `true`, ou `false` numa falha |
| `%%event%%` | o que aconteceu, por exemplo `container.failed` |
| `%%severity%%` | `info`, `success`, `warning`, `failure` ou `critical` |
| `%%importance%%` | o nível que o Unraid usa para isso: `normal`, `warning` ou `alert` |
| `%%link%%` | um link que pertence à mensagem, vazio quando não há nenhum |
| `%%key%%` | a secção, por exemplo `container` ou `VM` |
| `%%time%%` | a hora em UTC, por exemplo `2026-11-09T03:00:00Z` |
| `%%source%%` | `BombVault` |

Os marcadores também funcionam no endereço e nos valores dos cabeçalhos, e cada valor é escapado conforme o sítio onde está: como JSON quando o cabeçalho `Content-Type` diz JSON, e como parâmetro de consulta no endereço. Um webhook novo começa com esse cabeçalho e envia `{"title":"%%title%%","message":"%%body%%","ok":%%ok%%}`. Um nome que o BombVault não conhece fica como está escrito, por isso um erro de escrita nota-se na mensagem. Deixa o **Corpo** vazio para não enviar nenhum.

## Healthchecks {#healthchecks}

Um serviço como o [Healthchecks.io](https://healthchecks.io) dá o alarme quando o BombVault deixa de se apresentar. Junta um canal **Healthchecks** e cola o **URL de ping** da verificação, e o BombVault faz-lhe ping durante todo o ciclo de vida de cada backup: no início, depois no sucesso ou com `/fail`. Este canal não tem a escolha **Notificar**, porque uma verificação precisa dos seus pings de sucesso para ficar verde. Uma secção sem nenhum sítio para onde fazer backup conta como falhada, por isso a sua verificação fica vermelha em vez de receber um sucesso.

As secções com o mesmo URL partilham uma verificação. Depois de um backup de uma delas falhar, os backups bem-sucedidos das outras também fazem ping a `/fail` e indicam-na, por isso a verificação fica vermelha até essa entrada voltar a ter um backup bem-sucedido ou até confirmares a falha em **Precisa de atenção**.

Uma secção pode fazer ping a um endereço próprio: **Exceção** no canal Healthchecks junta um, e a linha passa a dizer **Todas as outras secções fazem ping a este URL.** **Enviar teste** escreve uma entrada no registo de cada verificação, o que deixa o estado dela como está.

## Exemplo: o Gotify no telemóvel {#example-gotify}

Tens o Gotify em `https://gotify.example.com` e queres que as falhas toquem no telemóvel enquanto os sucessos chegam em silêncio.

1. No Gotify, abre **Apps**, cria uma aplicação chamada `BombVault` e copia o seu token.
2. No BombVault, abre **Definições, Notificações** e liga **Resumir execuções agendadas**, para que uma noite de 45 backups envie uma mensagem em vez de 45.
3. Em **Canais**, carrega em **Adicionar canal** e escolhe **Gotify**.
4. Escreve `https://gotify.example.com` como **URL do servidor**. O BombVault acrescenta `/message` sozinho.
5. Cola o token em **Token da aplicação**.
6. Põe **Notificar** em **Em sucesso e falha**. Deixa a **Prioridade em caso de falha** em 8 e a **Prioridade em caso de sucesso** em 5. A aplicação Android do Gotify trata o 8 e acima como alta, por isso só as falhas fazem barulho.
7. Liga o canal e carrega em **Enviar teste**. A linha por baixo do canal diz se o Gotify o aceitou, e um token recusado é nomeado como tal.

A próxima execução agendada envia um resumo com prioridade 5, ou 8 se algo nela tiver falhado.

## Exemplo: o ntfy sem conta {#example-ntfy}

O ntfy não precisa de conta para um tópico público. Escolhe um nome de tópico difícil de adivinhar, porque quem o souber pode lê-lo.

1. Em **Canais**, carrega em **Adicionar canal** e escolhe **ntfy**.
2. Escreve o endereço do tópico como **URL do tópico**, por exemplo `https://ntfy.sh/bombvault-tower-7f3k`, ou um tópico no teu próprio servidor ntfy. Deixa o **Token de acesso (opcional)** vazio para um tópico público.
3. Subscreve o mesmo tópico na aplicação ntfy, liga o canal e carrega em **Enviar teste**.

O BombVault envia a mensagem como corpo, com o título no cabeçalho `Title`. Uma falha sai com prioridade alta e a etiqueta `warning`, para se destacar na aplicação, e uma VM que ficou congelada sai como urgente.

## Quando as mensagens não chegam {#troubleshooting}

Vê [As notificações não chegam](troubleshooting.md#notifications-do-not-arrive) em Resolução de problemas.
