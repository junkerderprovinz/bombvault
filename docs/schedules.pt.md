# Agendamentos

**Definições, Agendamento** guarda todos os planos: o backup total que cobre tudo, um plano próprio para uma secção, as cópias externas, que entradas participam e por que ordem. Uma entrada pode ter um plano só dela na sua página. Esta página explica como se encaixam e percorre uma configuração típica.

## Planos que se leem como frases {#plans}

Cada plano é um botão que se lê como uma frase, por exemplo *todos os dias às 03:00* ou *a cada 2 dias às 01:30*. Um clique abre-o com cinco modos:

| Modo | O que defines |
|---|---|
| **Desativado** | Nada corre neste plano. |
| **Diário** | **Hora** |
| **Semanal** | **Dias** e **Hora** |
| **A cada N dias** | **A cada** tantos **dias**, e a **Hora**. Conta a partir da última execução agendada, por isso fazer backup de uma entrada à mão não empurra o plano para trás. |
| **Cron** | Uma **Expressão** de cinco campos, por exemplo `0 */6 * * *` para a cada 6 horas. **Exemplos** oferece alguns, e a janela mostra as próximas execuções. |

Todas as horas estão no fuso horário do servidor. No Unraid é o do próprio sistema. Noutros sítios, define `TZ`, ou todos os planos correm em UTC, vê [Variáveis de ambiente](configuration.md#environment-variables).

## Semana {#week}

O cartão **Semana** desenha todas as execuções planeadas de segunda a domingo, uma linha por dia: backups, cópias externas e verificações. Aponta para um traço para ler o que começa aí. Onde três planos começam em duas horas, o trecho fica marcado, para os poderes espaçar.

![Definições, Agendamento: a faixa Semana em cima, depois o backup total e os planos por secção.](assets/screenshots/schedule/week-dark.png#only-dark)
![Definições, Agendamento: a faixa Semana em cima, depois o backup total e os planos por secção.](assets/screenshots/schedule/week-light.png#only-light)

## Backup total {#full-backup}

O **Backup total** é o plano predefinido para tudo. **Quando os backups correm** define-o: uma passagem que copia cada secção por sua vez, containers, VMs, flash, pastas, conjuntos de dados ZFS e o auto-backup no fim. A linha por baixo diz que secções inclui. **Desativado** desliga-o, e então só corre o que tem um plano próprio.

- **Recuperar backups perdidos após o arranque**: quando o servidor estava desligado a uma hora planeada, o BombVault faz esse backup cerca de dois minutos depois de arrancar.
- **Comandos antes e depois**: o **Comando antes do backup total** corre no servidor com `sh -c` antes de a passagem começar, por exemplo um script que pára um serviço. Se falhar, o backup total corre na mesma. O **Comando depois do backup total** corre uma vez depois de todas as secções terem sido tentadas, mesmo que uma tenha falhado, o que serve para um ping que confirma que o servidor inteiro está protegido. Os comandos para um só container estão na página dele.

**Fazer backup de tudo agora** na página Backups inicia a mesma passagem à mão, sobre todas as secções ligadas.

## Planos por secção {#plans-per-section}

Uma secção pode sair do backup total e correr à sua hora. Em **Agendamentos por secção**, **Exceção** junta uma secção com um plano próprio, e **Remover exceção** devolve-a. A linha no cartão diz o que fazem as outras, por exemplo **Todas as outras secções correm no backup total.**

Uma secção com plano próprio não é copiada outra vez no backup total, por isso nada é copiado duas vezes. O mesmo vale para uma única entrada com plano próprio.

**Cópias externas por secção** funciona da mesma maneira para as cópias. Por predefinição cada secção copia para os seus destinos externos logo a seguir a cada backup. **Exceção** dá a uma secção um plano de cópia próprio, por exemplo para as VMs copiarem para o Backblaze B2 só nas noites de domingo. O cartão **Copiar** de um destino também pode definir **Quando copiar**, vê [Cópias externas](offsite-recovery.md#off-site-copies).

## Entradas agendadas {#scheduled-entries}

**Entradas agendadas** diz por secção quantas entradas participam, por exemplo **38 de 41 agendadas**. **Entradas** abre a lista com um interruptor **No agendamento** por entrada. Uma entrada desligada só é copiada quando a inicias à mão. Conta como não protegida, a menos que a marques com **Não fazer backup**.

## Ordem {#order}

**Ordem** define a ordem pela qual as execuções agendadas e o backup total copiam containers e VMs. Arrasta uma entrada para o lugar ou usa as setas, e cada movimento é guardado logo. As entradas que deixares sem ordem são copiadas pela mais atrasada primeiro. Um backup único à mão não é afetado.

**Reiniciar após o backup** no mesmo cartão trata dos containers que **Parar outros contentores** parou durante um backup. Arrancam outra vez pela ordem das suas dependências, e com **Aguardar que as dependências estejam saudáveis antes de iniciar o seguinte** cada um tem de se declarar saudável ou em execução antes de o seguinte arrancar. **Tempo limite de saúde por contentor (segundos)** limita a espera, 120 por predefinição, num intervalo de 5 a 3600.

## Instantâneo em vez de parar {#snapshot-instead-of-stop}

**Parar os contentores em ZFS só para um instantâneo** vem ligado por predefinição. Um container cujos dados estão em ZFS só é parado enquanto o BombVault tira um instantâneo desses dados, normalmente alguns segundos, e o backup é então lido do instantâneo. Precisa da ligação SSH ao host que os conjuntos de dados ZFS usam. Um container com parte dos dados noutro sítio é parado durante todo o backup como antes, e a sua execução di-lo, enquanto um container sem nenhum dado em ZFS é parado sem nota. Funciona numa partilha de utilizador do Unraid que esteja num só pool ZFS. No TrueNAS a aplicação precisa também da capacidade `SYS_CHROOT`, vê [TrueNAS SCALE](getting-started.md#truenas). Um só container pode ser configurado de outra forma na sua própria página.

O backup fica sob os mesmos caminhos e etiquetas de um feito com o container parado, por isso as restaurações e as verificações tratam os dois por igual. O comando antes do backup e o despejo da base de dados correm primeiro na mesma, os outros containers a parar descem e voltam com ele, e o comando depois do backup corre assim que o container está outra vez de pé. Uma atualização depois do backup continua a esperar até o backup terminar. As suas execuções mostram **Instantâneo** e quantos segundos o container esteve em baixo. Se o BombVault parar a meio de qualquer backup de container, volta a arrancar os containers que tinha parado na próxima vez que arrancar.

## Atualizações após o backup {#updates}

- **Atualizar contêineres após um backup bem-sucedido** só transfere uma imagem mais recente depois de o backup anterior ter terminado, por isso existe sempre primeiro um ponto de restauro novo. Vem desligado por predefinição, e um só container pode ser configurado de outra forma na sua página em **Atualizar após um backup bem-sucedido**.
- **Remover a image antiga após uma atualização** elimina a imagem que o container deixou. Uma imagem de base que outros containers partilham nunca é eliminada.
- **Atualizar o estado de atualizações do Unraid depois de atualizar um contentor** limpa o aviso de atualização do Unraid para esse container.

Uma mensagem por container atualizado define-se em **Definições, Notificações** com **Notificar ao atualizar container**.

## Inatividade antes do backup {#idle}

O cartão **Inatividade antes do backup** define quando uma aplicação conta como inativa para um container cujo backup agendado espera por isso: **CPU abaixo de (% de um núcleo)**, **Tráfego abaixo de (Mbit/s)** e **Durante pelo menos (min)**. Um servidor multimédia conta como inativo quando não está a transmitir, e para um container na rede do host só conta o CPU. A espera em si define-se em cada container, vê [Quando a aplicação está ocupada](backups.md#app-busy).

## Um plano próprio para uma entrada {#own-plan}

Cada entrada tem **Plano próprio** no cartão **Agendamento e destino** do seu separador Definições. Com um plano aí, a entrada corre às suas próprias horas e sai do backup total e do plano da sua secção. Sem ele, segue-os.

## Exemplo: três planos que se encaixam {#example}

Uma caixa Unraid típica: a maioria dos containers pode ser copiada de noite, as VMs são grandes e mudam devagar, e o Immich recebe fotografias novas o dia todo.

1. **Backup total às 03:00.** Abre **Definições, Agendamento**. Em **Backup total**, põe **Quando os backups correm** em **Diário** às `03:00`. Liga **Recuperar backups perdidos após o arranque**, para que uma noite com o servidor desligado seja compensada.
2. **VMs uma vez por semana.** Em **Agendamentos por secção**, carrega em **Exceção**, escolhe **VMs** e põe o seu plano em **Semanal**, **Dias** domingo, **Hora** `04:00`. O cartão passa a dizer que todas as outras secções correm no backup total.
3. **As VMs copiam para o exterior também ao domingo.** Em **Cópias externas por secção**, carrega em **Exceção**, escolhe **VMs** e põe **Semanal**, domingo às `06:00`, depois de o backup ter tido tempo de acabar.
4. **Immich a cada 6 horas.** Abre **Backups**, depois `immich`, depois o separador **Definições**. Põe **Plano próprio** em **Cron** com `0 */6 * * *`.

O que corre agora:

| Quando | O quê |
|---|---|
| Todos os dias às 03:00 | Containers exceto o `immich`, flash, pastas, conjuntos de dados ZFS e o auto-backup, uma secção depois da outra |
| Domingo às 04:00 | VMs |
| Domingo às 06:00 | A cópia externa das VMs |
| 00:00, 06:00, 12:00 e 18:00 | `immich` sozinho |

O cartão **Semana** mostra a linha das 03:00 em todos os dias, os traços das 04:00 e das 06:00 ao domingo, e quatro traços por dia para o `immich`. O `immich` não é copiado uma segunda vez às 03:00, porque uma entrada com plano próprio sai do backup total. As cópias das outras secções continuam a correr logo a seguir a cada backup.

Quando mais tarde removeres o plano próprio do `immich`, ele volta a juntar-se à execução das 03:00 com a sua secção.
