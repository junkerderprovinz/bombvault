# Visão geral

A visão geral é a primeira página da barra lateral e a que vale a pena ver todas as manhãs. Responde a duas perguntas num relance: está tudo com backup, e há alguma coisa que precise de ti? Começa com um cabeçalho de estado e quatro cartões, e há mais cartões à espera em **Personalizar**.

![A visão geral passados alguns dias: o cabeçalho de estado, Precisa de atenção com um aviso, a proteção de cada secção, os destinos de armazenamento e o histórico.](assets/screenshots/overview/home-dark.png#only-dark)
![A visão geral passados alguns dias: o cabeçalho de estado, Precisa de atenção com um aviso, a proteção de cada secção, os destinos de armazenamento e o histórico.](assets/screenshots/overview/home-light.png#only-light)

## O cabeçalho de estado {#status-head}

O cabeçalho fica no topo da página e está sempre lá. O anel mostra quanto está protegido, por exemplo **41 de 43 protegidos**, e a linha ao lado diz como estão as coisas:

| O cabeçalho diz | O que significa |
|---|---|
| **Tudo copiado** | Todas as entradas de todas as secções ligadas têm um backup atual. |
| **Copiado, com avisos** | Está tudo com backup, e algumas execuções deixaram uma nota, por exemplo ficheiros que não conseguiram ler. |
| **Nem tudo está protegido ainda** | Algumas entradas não têm um backup atual, por exemplo porque esperam pelo primeiro. |
| **3 coisas precisam de ti** | Algo falhou ou espera por uma decisão. O cartão **Precisa de atenção** lista o que é. |
| **A fazer backup agora** | Está a correr um backup. O anel passa a um anel de progresso com as estações **Parar**, **Ler** e **Iniciar**, e um backup completo conta as entradas à medida que avança, por exemplo **A fazer backup agora: 12 de 43**. **Ver execução** abre a execução. |
| **Cópia externa em curso** | Está a correr uma cópia para um destino de armazenamento. Não mostra percentagem, porque o `restic copy` não indica nenhuma. |

Por baixo da linha vê-se quando foi a última execução e quando começa a próxima planeada.

## Precisa de atenção {#needs-you}

Este cartão lista o que deves ver, do mais recente para o mais antigo, e diz **Nada a fazer.** quando a lista está vazia. Cada linha tem a sua própria saída:

- **Um backup falhado** indica a entrada e o motivo. **Ver erros** mostra a saída do próprio restic, e **Tentar de novo** volta a iniciar o backup.
- **Falhou a cópia externa** indica a secção e desde quando. **Replicar agora** tenta outra vez.
- **Anomalia em plex** leva ao achado com **Verificação**. Enquanto houver um achado crítico em aberto, a retenção mantém os backups antigos dessa entrada.
- As entradas **Novo** que o BombVault encontrou e que ainda nada copia, por exemplo um container que acabaste de instalar, oferecem **Começar a fazer backup** ou **Não fazer backup**.
- **Um backup que desistiu** porque a aplicação ficou ocupada durante toda a espera, vê [Quando a aplicação está ocupada](backups.md#app-busy).
- **Uma réplica ZFS falhada**, em palavras simples, por exemplo que o servidor não tem o comando zfs, com a saída do servidor em **Ver**.
- **Os backups estão parados: a chave já não se consegue abrir** aparece quando este servidor assumiu a chave de outro servidor e o `APP_KEY` mudou depois. Vê [Resolução de problemas](troubleshooting.md#backups-stopped-and-say-the-key-can-no-longer-be-opened).
- **Guarda o teu kit de recuperação** fica até transferires o kit e carregares em **Já o guardei em segurança**.

## Estado de proteção {#protection-status}

Cada secção ligada tem uma linha com quatro verificações:

| Verificação | Fica verde quando | Outras palavras que podes ver |
|---|---|---|
| **Local** | o último backup está atual | ainda sem backup, sem agendamento, a última execução falhou, backup em atraso, nem todas as entradas estão protegidas, backup a vencer em breve |
| **Externo** | a última cópia fora das instalações está atual | sem cópia externa, cópia em curso, cópia em atraso, cópia em pausa, ainda sem cópia, réplica falhada |
| **Proteção contra eliminação** | um teste de adulteração provou que o outro lado recusa eliminações | nenhuma, nenhuma e risco aceite, ainda não provada, teste falhado, não provada porque a verificação está desligada |
| **Restauro de teste** | passou um restauro a partir da cópia externa | nunca executado, em curso, falhado, em atraso, só manual, não é possível para esta secção |

Uma secção só fica verde com factos verificados. Uma cópia neste servidor não conta como externa, e um destino que marcaste com **Aceitar sem proteção contra eliminação** mostra um anel cinzento em vez de um ponto âmbar, por isso a cor da secção segue as suas outras verificações.

Um clique numa secção abre a sua janela com todas as verificações explicadas, as entradas que conta e **Iniciar o restauro de teste** ou **Executar o restauro de teste novamente**.

![A janela de proteção da secção Containers com as suas quatro verificações explicadas.](assets/screenshots/overview/protection-window-dark.png#only-dark)
![A janela de proteção da secção Containers com as suas quatro verificações explicadas.](assets/screenshots/overview/protection-window-light.png#only-light)

## Destinos de armazenamento {#storage-locations}

Cada destino de armazenamento aparece aqui como um recipiente com o seu nível de enchimento, a velocidade a que cresce, por exemplo **A crescer 4,2 GB/semana**, e quando ficaria cheio a esse ritmo, por exemplo **cheio em cerca de 31 semanas**. O S3, o B2 e os rest-servers não indicam quanto espaço resta, por isso mostram **Espaço livre desconhecido** e só o crescimento. Um clique leva à página do destino. Vê [Destinos](offsite-recovery.md#destinations).

## Histórico {#history}

O **Histórico** tem dois separadores. **Execuções** lista todas as execuções com o resultado, a duração e o motivo, e um clique abre-a. **Registo** mostra cada linha com a sua hora, tal como o BombVault a escreve, e pode ser reduzido a um dia. Uma linha fica laranja quando um backup correu mas deixou alguma coisa de fora, e a linha diz o que deixou.

## Mais cartões em Personalizar {#customize}

**Personalizar** no topo da página deixa-te organizar os cartões. Arrasta um cartão pela barra para o mover, muda-lhe o tamanho no ponto do canto inferior ou oculta-o. Os cartões ocultos esperam em **Cartões ocultos**, e **Mostrar** traz um de volta. Enquanto os organizas, os cartões abanam um pouco, e **Concluído** termina. A disposição fica guardada neste navegador.

Estes cartões começam ocultos:

- **Anomalias**: os achados em aberto, vê [Anomalias](anomalies.md).
- **Sem backup**: tudo o que está neste servidor e nenhum backup agendado cobre, cada um com o seu motivo, por exemplo **nunca adicionado**, **não incluído no agendamento** ou **todos os destinos da sua secção estão desligados**. Lê os containers diretamente do Docker, por isso encontra também um que o BombVault nunca viu.
- **Calendário**: os últimos 90 dias, um quadrado por dia. Um clique num dia mostra as suas linhas no registo em **Histórico**.
- **Armazenamento**: quanto ocupa cada secção e quão bem os seus dados se deduplicam.
- **Visão geral**: o snapshot mais antigo de todas as secções, quando corre o próximo backup e como correu o último.
- **Estatísticas**: quantos containers e VMs existem, quantos erros, e quantos planos estão ativos ou em pausa.
- **Integração com o host**: o que o BombVault alcança e precisa no host, como o socket Docker, o libvirt, o restic e o rclone. Começa aqui quando algo parece não estar ligado.

![O modo Personalizar: os cartões podem ser arrastados, redimensionados e ocultados, e os ocultos esperam em baixo.](assets/screenshots/overview/customize-dark.png#only-dark)
![O modo Personalizar: os cartões podem ser arrastados, redimensionados e ocultados, e os ocultos esperam em baixo.](assets/screenshots/overview/customize-light.png#only-light)

## Como o BombVault conta {#counting}

Todos os cartões e o cabeçalho contam da mesma maneira: as entradas das secções ligadas que ainda estão instaladas e que não marcaste com **Não fazer backup**. Uma entrada só conta como protegida enquanto um agendamento a alcança e ela tem um backup. Um backup antigo, sozinho, não conta, por isso um container que tiraste do agendamento aparece como não protegido até o voltares a pôr ou o marcares com **Não fazer backup**.

## Primeiros passos {#first-steps}

Numa instalação nova, a visão geral começa com o cartão **Primeiros passos**: **Escolhe as secções**, **Adicionar destino** e **Primeiro backup**. Fica até o primeiro backup terminar. A [Introdução](getting-started.md#first-run) acompanha-te por ele.
