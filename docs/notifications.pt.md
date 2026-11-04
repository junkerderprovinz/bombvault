# Notificações

O BombVault pode avisar-te quando um backup falha, quando tem êxito, quando há um em atraso, e uma vez por semana como correram as coisas. Tudo se define em **Definições, Notificações**: o que dispara uma mensagem no cartão **Notificações**, para onde vão as mensagens no cartão **Canais**, e os pings de presença no cartão **Healthchecks**.

![Definições, Notificações com o canal Gotify preenchido.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Definições, Notificações com o canal Gotify preenchido.](assets/screenshots/notifications/gotify-light.png#only-light)

## O que envia uma mensagem {#what}

O cartão **Notificações** define as regras para todos os canais de uma vez:

- **Notificar**: **Nunca**, **Apenas em caso de falha** ou **Em sucesso e falha**, para todos os canais abaixo.
- **Resumir execuções agendadas**: uma mensagem por execução agendada, por exemplo *42 de 45 com êxito*, em vez de uma por container ou VM. Um backup que inicias à mão continua a enviar a sua própria mensagem. Um convidado congelado, uma retenção falhada ou um orçamento de crescimento ultrapassado seguem logo, mesmo com os resumos ligados.
- **Notificar ao atualizar container**: uma mensagem por cada container que **Atualizar contêineres após um backup bem-sucedido** passou para uma imagem mais recente, para poderes ver se continua a funcionar.
- **Comunicar backups em atraso**: uma vez por dia, às 09:00, o BombVault verifica se algum backup ligado está em atraso, ou seja, mais velho do que o dobro do seu agendamento, e envia uma mensagem por caso. O backup bem-sucedido seguinte limpa-o.
- **Notificações do Unraid**: envia para o sistema de notificações do próprio Unraid, que pode reencaminhar para o Pushover, e-mail ou Discord. Corre pela ligação SSH de **Definições, Integrações, SSH do anfitrião**, por isso essa chave tem de estar autorizada. No TrueNAS e noutros hosts fica desligada.
- **Resumo semanal**: um resumo por semana com o número de execuções, os novos dados de backup, se as cópias externas estão atuais e as principais falhas. O seu plano é um botão como todos os outros planos, e **Desativado** desliga-o. Vê [Agendamentos](schedules.md#plans).

Que anomalias enviam uma mensagem define-se em **Definições, Verificação**, vê [Anomalias](anomalies.md). Um backup adiado envia uma mensagem quando começa a esperar e outra quando a espera termina, não uma por tentativa.

**Enviar teste**, no fundo do cartão, envia uma mensagem de teste por todos os canais que estão ligados, com os valores tal como estão no formulário.

## Canais {#channels}

Cada canal tem o seu próprio interruptor no cartão **Canais**, e os seus campos aparecem quando está ligado. Cada canal envia por si, com o seu próprio limite de tempo, por isso um host que nunca responde não atrasa os outros, e uma mensagem que nenhum canal aceitou é tentada outra vez.

| Canal | Campos | Notas |
|---|---|---|
| **Webhook** | **URL do Webhook**, **Formato do Webhook** | Formatos: **JSON genérico**, **Discord**, **Slack**, **Gotify** e **ntfy**. O JSON genérico envia `title`, `message` e `ok`. |
| **Apprise** | **URL da API Apprise**, **Etiquetas (opcional)** | Aponta-o para o endpoint `/notify/<key>` do teu container Apprise API para chegar aos seus mais de 100 serviços. As etiquetas escolhem um subconjunto dos destinos dessa chave. |
| **Matrix** | **URL do Homeserver**, **Token de acesso**, **ID da sala** | O token fica no servidor e nunca mais é mostrado. |
| **Gotify** | **URL do servidor**, **Token da aplicação**, **Prioridade em caso de falha**, **Prioridade em caso de sucesso** | As prioridades vão de 0 a 10 e começam em 8 para falhas e 5 para sucessos. |
| **E-mail (SMTP)** | **Host SMTP**, **Porta**, **Encriptação**, **Nome de utilizador**, **Palavra-passe**, **Endereço de origem**, **Endereço de destino** | A **Encriptação** é **STARTTLS**, **TLS (implícito)** ou **Nenhuma**. |

Os segredos, como o token do Matrix, o token da aplicação do Gotify e a palavra-passe do SMTP, ficam guardados encriptados. Por baixo de um segredo guardado o campo diz **Guardado. Deixe o campo vazio para o manter.** Quando mudas o endereço do servidor, o BombVault não envia o segredo antigo para o novo sítio: o campo pede-o outra vez, e o novo endereço fica guardado quando o escreves.

## Healthchecks {#healthchecks}

Um serviço como o [Healthchecks.io](https://healthchecks.io) dá o alarme quando o BombVault deixa de se apresentar. Cola o **URL de ping** da verificação, e o BombVault faz-lhe ping durante todo o ciclo de vida de cada backup: no início, depois no sucesso ou com `/fail`. Fá-lo sempre que há um URL definido, seja o que for que **Notificar** diga, por isso a verificação fica verde no sucesso mesmo com mensagens só de falha.

Uma secção pode fazer ping a um endereço próprio: **Exceção** no cartão junta um, e a linha diz **Todas as outras secções fazem ping a este URL.**

## Exemplo: o Gotify no telemóvel {#example-gotify}

Tens o Gotify em `https://gotify.example.com` e queres que as falhas toquem no telemóvel enquanto os sucessos chegam em silêncio.

1. No Gotify, abre **Apps**, cria uma aplicação chamada `BombVault` e copia o seu token.
2. No BombVault, abre **Definições, Notificações**. Em **Notificações**, põe **Notificar** em **Em sucesso e falha** e liga **Resumir execuções agendadas**, para que uma noite de 45 backups envie uma mensagem em vez de 45.
3. Em **Canais**, liga o **Gotify**.
4. Escreve `https://gotify.example.com` como **URL do servidor**. O BombVault acrescenta `/message` sozinho.
5. Cola o token em **Token da aplicação**.
6. Deixa a **Prioridade em caso de falha** em 8 e a **Prioridade em caso de sucesso** em 5. A aplicação Android do Gotify trata o 8 e acima como alta, por isso só as falhas fazem barulho.
7. Carrega em **Enviar teste**. A linha por baixo do cartão diz se o Gotify o aceitou, e um token recusado é nomeado como tal.

A próxima execução agendada envia um resumo com prioridade 5, ou 8 se algo nela tiver falhado.

## Exemplo: o ntfy como webhook {#example-ntfy}

O ntfy não precisa de conta para um tópico público. Escolhe um nome de tópico difícil de adivinhar, porque quem o souber pode lê-lo.

1. Em **Canais**, liga o **Webhook**.
2. Escreve o endereço do tópico como **URL do Webhook**, por exemplo `https://ntfy.sh/bombvault-tower-7f3k`, ou o endereço do teu próprio servidor ntfy.
3. Põe o **Formato do Webhook** em **ntfy**.
4. Subscreve o mesmo tópico na aplicação ntfy e carrega em **Enviar teste**.

O BombVault envia a mensagem como corpo, com o título em `X-Title`. Uma falha sai com prioridade alta e a etiqueta `warning`, para se destacar na aplicação.

## Quando as mensagens não chegam {#troubleshooting}

Vê [As notificações não chegam](troubleshooting.md#notifications-do-not-arrive) em Resolução de problemas.
