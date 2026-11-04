# Anomalias

O ransomware e uma aplicação avariada parecem-se num backup: de repente quase todos os dados são novos, ou quase todos desapareceram. O BombVault compara cada backup com o historial da própria entrada e levanta um achado quando algo parece errado. Enquanto um achado sobre dados perdidos está em aberto, mantém também os backups antigos da entrada, para que o último bom não possa envelhecer até desaparecer. A entrada **Anomalias** na barra lateral aparece enquanto a deteção está ligada.

![A página Anomalias com um achado crítico em aberto.](assets/screenshots/anomalies/page-dark.png#only-dark)
![A página Anomalias com um achado crítico em aberto.](assets/screenshots/anomalies/page-light.png#only-light)

## A página {#page}

Os mosaicos **Críticas**, **Avisos** e **Notas** escolhem uma gravidade, e **Abertas** e **Fechadas** por cima da lista alternam entre o que está em aberto e o que fechou nos últimos 30 dias. Um achado crítico fica na mesma superfície que os outros, e a sua gravidade aparece no selo e na borda colorida. Um achado oferece primeiro a sua solução, e os detalhes mostram o valor como uma curva sobre o intervalo habitual.

Quando nada está em aberto, o **Progresso da aprendizagem** mostra um anel de dez partes por entrada, uma parte por cada backup de que o BombVault aprendeu. **Monitorização** abre a lista de entradas com as suas próprias definições.

## O que verifica {#checks}

Cada backup de um container, VM, conjunto de pastas, despejo da base de dados, da unidade flash e do auto-backup é comparado com o historial da própria entrada. As verificações olham para:

- os dados novos que uma execução guardou, contra as maiores quantidades habituais de backups recentes e contra a taxa habitual por hora,
- um backup que voltou a guardar a maior parte dos dados, o que inclui ficheiros que foram renomeados e reescritos,
- o tamanho da origem e o número de ficheiros que o restic indica para cada entrada e cada despejo da base de dados,
- o tempo de backup do próprio restic,
- sequências de falhas e falhas intermitentes,
- verificações de restauro que deixaram de passar,
- o espaço livre dos repositórios locais, SFTP e rclone, projetado a partir do crescimento do repositório.

Uma entrada aprende com os seus primeiros 10 backups. Uma origem quase vazia, a reescrita da maior parte dos dados e as falhas são verificadas desde o início.

Um achado sobre o tamanho ou o número de ficheiros de uma origem é datado do primeiro backup em que apareceu. **Comparar com o backup anterior** lista as pastas onde ficheiros desapareceram, entraram ou mudaram, com uma nota quando quase tudo está num índice de pesquisa, numa cache ou em miniaturas que a aplicação reconstrói sozinha.

As entradas ZFS são vigiadas conjunto de dados a conjunto de dados, vê [Conjuntos de dados ZFS](zfs-datasets.md#anomalies). Um assistente pode ler os achados em aberto através do [servidor MCP](mcp.md#tools).

As verificações têm limites. Os repositórios S3, B2 e REST não têm valor de espaço livre, na partilha de utilizador do Unraid o espaço livre é o de todo o array, e os backups feitos antes do restic 0.17 não trazem historial de tamanhos.

## Fechar um achado {#closing}

- Os **avisos** fecham-se sozinhos quando a causa desaparece.
- Os achados **críticos** sobre dados perdidos e um disco a encher ficam até agires.
- **Confirmar** marca um achado como visto e fecha-o. Não volta a ser levantado até a sua causa ter desaparecido uma vez.
- **Marcar como esperada** diz que a mudança foi tua. O novo nível passa a normal após 10 backups, mas a verificação de quase vazio nunca é desligada.
- Uma seleção alterada, por exemplo uma pasta que marcaste ou desmarcaste, recomeça sozinha o historial da entrada.

Uma notificação sai uma vez por episódio, e as falhas e as verificações de restauro que já notificam não são comunicadas duas vezes.

## Entretanto os backups antigos ficam guardados {#retention-hold}

Enquanto uma origem está quase vazia, encolheu muito ou teve a maior parte dos dados guardada outra vez, a retenção e a poda deixam em paz os backups antigos dessa entrada, e o achado liga ao último bom backup, que a janela de restauração pré-seleciona. Antes de fechares um achado destes, o BombVault diz que a próxima limpeza voltará a eliminar backups antigos segundo as regras de retenção. Por isso, se faltam dados ou foram encriptados, restaura primeiro e fecha o achado depois.

## Definições {#settings}

O cartão **Anomalias** em **Definições, Verificação** guarda as definições globais. Cada controlo guarda-se assim que o mudas, e os três por baixo do interruptor ficam escondidos enquanto a deteção está desligada.

| Definição | Predefinição | O que faz |
|---|---|---|
| **Detetar anomalias** | Ligado | Compara cada backup com o historial da própria entrada. Desligado, nada de novo é verificado e a entrada **Anomalias** sai da barra lateral. O cartão continua a ligar aos achados anteriores. |
| **Sensibilidade** | Equilibrada | A rigorosa comunica mudanças menores, a permissiva só as grandes. |
| **Enviar uma notificação para** | Só achados críticos | A gravidade mais baixa que envia uma mensagem pelos canais configurados em [Notificações](notifications.md). As falhas repetidas de backup e de despejo e as verificações de restauro agendadas falhadas já enviam a sua própria mensagem e não são enviadas duas vezes. |
| **Manter os backups antigos quando uma origem encolhe muito ou é reescrita** | Ligado | Enquanto uma entrada tem um achado em aberto por uma origem quase vazia, um encolhimento brusco ou a maior parte dos dados guardada outra vez, a retenção e a poda deixam em paz os backups antigos dessa entrada. Confirma o achado ou marca-o como esperado para os deixar ir. |

Cada entrada pode usar a sua própria sensibilidade e o seu próprio mínimo de notificação. Define-os no separador **Verificação** da entrada, ou na página **Anomalias** em **Monitorização**. Para uma entrada ZFS aplicam-se a todos os conjuntos de dados da sua árvore.
