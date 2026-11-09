# Backups e entradas

Tudo o que o BombVault copia está numa só lista em **Backups**: containers, VMs, conjuntos de pastas, a unidade flash, conjuntos de dados ZFS e o auto-backup. Cada um deles é uma **entrada** com uma página própria. Esta página percorre a lista, a página da entrada e os seus três separadores, e as definições que uma entrada pode ter.

![A lista de Backups com o mosaico Containers escolhido: cada linha mostra as suas últimas execuções numa faixa e um estado.](assets/screenshots/backups/list-dark.png#only-dark)
![A lista de Backups com o mosaico Containers escolhido: cada linha mostra as suas últimas execuções numa faixa e um estado.](assets/screenshots/backups/list-light.png#only-light)

## O que cada secção guarda {#what-is-saved}

| Secção | O que é guardado |
|---|---|
| **Containers** | A pasta appdata mais a definição do container: imagem, variáveis, portas, etiquetas e volumes. O cartão **Pastas a copiar** no separador **Definições** do container marca exatamente as pastas que o backup cobre, com uma contagem em direto dos caminhos e uma lista do que deixaste de fora. Os dados que um container guarda fora do Host Data, por exemplo num ixVolume do TrueNAS ou num projeto Compose em `/opt`, não são copiados, e a execução diz que pastas deixou de fora. A execução também indica uma pasta dentro do Host Data que neste momento não existe. Caminhos de sistema como `/etc/localtime` ou o socket do Docker não contam como dados. |
| **VMs** | As imagens de disco, a definição XML e a NVRAM UEFI, com um encerramento ordenado ou um instantâneo a quente, por SSH. Vê [Backups de VMs](vm-backup-ssh-setup.md). |
| **Flash** | Só no Unraid. Toda a flash USB (`/boot`): sistema, licença, configuração do array, partilhas, rede e configuração dos plugins. |
| **Auto-backup** | O `/config` do próprio BombVault: a sua base de dados de definições, os inícios de sessão dos destinos de armazenamento, o par de chaves SSH do libvirt, a chave e as chaves de anfitrião fixadas dos servidores ZFS e os certificados, captados com `VACUUM INTO` do SQLite para que a base de dados nunca seja copiada a meio de uma escrita. |
| **Pastas** | Qualquer pasta do servidor como conjunto de pastas, por exemplo uma partilha, os teus documentos ou uma fototeca, com padrões de exclusão opcionais. Os conjuntos de pastas têm agendamentos, retenção, cópias externas, verificações e testes de restauro como tudo o resto. |
| **Conjuntos de dados ZFS** | Um conjunto de dados e todos os que estão por baixo dele, lidos a partir de um só instantâneo ZFS para que todos venham do mesmo instante, e guardados com o restic como uma pasta. Uma entrada ZFS pode também manter uma réplica exata ao bloco num segundo servidor ZFS. Vê [Conjuntos de dados ZFS](zfs-datasets.md). |

Todos os backups passam pelo restic, por isso são incrementais e deduplicados, e até discos de VM grandes não fazem o repositório inchar. O BombVault nunca copia, e por isso nunca pára, o seu próprio container. Uma execução pode ser parada em segurança, porque o restic escreve o seu instantâneo no fim, por isso uma execução abortada deixa dados sem referência e nenhum instantâneo. Um bloqueio deixado por um reinício a meio de uma execução é limpo e a execução tentada mais uma vez, e só depois de o bloqueio ter passado dez minutos sem ser renovado.

## A lista {#list}

Os mosaicos por cima da lista escolhem um tipo, por exemplo **Containers** ou **ZFS**, e cada mosaico conta as suas entradas. Um mosaico seleciona, não esconde o resto da página.

Cada linha mostra as últimas catorze execuções da entrada como pequenos quadrados e um estado, por exemplo **Protegido**, **Não protegido**, **Adiado** ou **A replicar**. Tudo o resto sobre ela está no balão de informação ao lado do nome, e enquanto o ponteiro está sobre a linha esse texto aparece na própria linha. Uma marca ao lado do nome de um container diz que foi recriado com outra imagem, outras portas, variáveis ou volumes desde o último backup. Desaparece com o backup seguinte.

Três botões por cima da lista restringem-na:

- **Filtros**: por **Agendamento** (**Todos**, **Agendado**, **Em pausa**), por **Backup** (**Com backup**, **Sem backup**) e, para **Containers e VMs**, **Instalados** ou **Não instalado**.
- **Ordenar**: por nome, estado, IP, último backup, tamanho ou agendamento. Todos os campos exceto o IP ordenam nos dois sentidos, por exemplo **Tamanho, maiores primeiro** e **Tamanho, menores primeiro**.
- **Pesquisar**: por nome, imagem ou IP.

As entradas que já não estão instaladas mas ainda têm backups ficam à parte em **Não instalado (apenas backups)**. Restaura-as, ou elimina os seus backups para libertar o espaço.

A ação principal da página flutua em baixo à direita: **Fazer backup de tudo agora** inicia uma execução sobre todas as secções ligadas. **Mais** guarda o resto:

- **Selecionar** inicia uma seleção, vê abaixo.
- **Descobrir backups** lê o que os destinos de armazenamento guardam e reconstrói entradas a partir daí, por exemplo depois de uma instalação nova. Um container sem dados próprios não tem instantâneo e volta a partir da definição espelhada junto ao repositório. As cópias externas das secções reconstruídas ficam então à espera até assumires os backups delas como deste servidor, vê [Depois de uma reconstrução](offsite-recovery.md#after-a-rebuild).
- **Ligação ZFS a este servidor** abre o teste SSH da secção ZFS, vê [Conjuntos de dados ZFS](zfs-datasets.md#requirements).
- **Replicar tudo agora**, com o mosaico ZFS escolhido, envia a réplica de todas as entradas ZFS.

![A mesma lista num telemóvel.](assets/screenshots/backups/list-phone-dark.png#only-dark)
![A mesma lista num telemóvel.](assets/screenshots/backups/list-phone-light.png#only-light)

## Escolher várias entradas {#select}

Escolhe linhas como escolhes ficheiros num gestor de ficheiros: Ctrl+clique, Shift+clique, Ctrl+A, ou um toque longo num telemóvel. Uma linha escolhida mostra um visto no lugar do ícone, e uma barra por cima da lista conta-as e oferece **Fazer backup da seleção**, **Pausar agendamento** e **Retomar agendamento**, **Selecionar tudo** e **Limpar seleção**. Cada linha tem também um menu de clique direito com as suas próprias ações.

Uma seleção é copiada no servidor, uma entrada depois da outra, por isso continua quando fechas o separador. Para restaurar várias entradas em conjunto, usa a [página Restauração](restore.md#several).

![Dois containers escolhidos, com a barra por cima da lista.](assets/screenshots/backups/bulk-dark.png#only-dark)
![Dois containers escolhidos, com a barra por cima da lista.](assets/screenshots/backups/bulk-light.png#only-light)

## Adicionar uma pasta ou um conjunto de dados ZFS {#add}

Os containers, as VMs e a unidade flash aparecem sozinhos quando a sua secção está ligada em **Definições, Geral**. As pastas e os conjuntos de dados ZFS escolhes tu, porque o BombVault não consegue adivinhar quais queres. **Adicionar** abre **Do que mais o BombVault deve fazer backup?**:

- **Uma pasta** lista as partilhas que o BombVault encontrou e aceita qualquer outra pasta dentro do Host Data. Dá um nome ao conjunto de pastas, por exemplo `Documents`. O nome fica fixo depois do primeiro backup, porque os instantâneos são guardados com ele.
- **Um conjunto de dados ZFS** lista os conjuntos de dados deste servidor. Escolhe o que está no topo do que queres copiar, vê [Entradas e conjuntos de dados filhos](zfs-datasets.md#items-and-children).
- **Configuração do sistema** aparece num host sem Unraid cujo Host Data chega a `/etc`. É um conjunto de pastas `/etc` inicial que revês antes de guardar.

## Deixar uma entrada de fora {#dont-back-up}

Uma entrada que deixas de fora de propósito, por exemplo um container de teste, pode ser marcada com **Não fazer backup**. Deixa então de contar como não protegida, e **Deixar de excluir** traz-a de volta. **Pausar agendamento** é a via mais suave: a entrada sai do agendamento e do backup completo, ganha um selo cinzento **Agendamento em pausa**, e **Fazer backup agora** continua a funcionar. Mexe no mesmo interruptor que **No agendamento** no separador Definições da entrada.

## A página de uma entrada {#entry-page}

Um clique numa linha abre a página da entrada. O cabeçalho nomeia a entrada e o seu estado numa linha, por exemplo **Último backup há 3 horas** ou **Adiado, próxima tentativa às 03:15**, e guarda as suas ações: **Fazer backup agora**, **Restauração**, e em **Mais** as raras, como **Exportar o backup mais recente**, **Exportar container** ou **Exportar VM**, **Associar a uma entrada anterior**, **Eliminar todos os backups**, **Remover entrada** e **Eliminar backups em todo o lado**. Um botão de voltar à esquerda do ícone leva à lista. Enquanto um backup corre, **Cancelar backup** fica ao lado do progresso, e uma execução cancelada fica registada como cancelada, não como falhada.

**Eliminar backups em todo o lado** apaga de uma vez os backups da entrada em todos os destinos, incluindo as cópias externas. A janela mostra cada lugar com quantos backups guarda, e o botão desbloqueia assim que escreves o nome da entrada. Um lugar configurado como append-only mantém os seus backups e diz isso. A entrada e as suas definições ficam, por isso uma entrada de que já não fazes backup continua assim, e só desaparecem os seus backups e o histórico deles.

A página tem três separadores: **Backups**, **Definições** e **Verificação**.

### Separador Backups {#backups-tab}

Este separador lista os instantâneos da entrada, do mais recente para o mais antigo, com uma barra por dia. Um backup que está em vários lugares aparece uma vez, marcado com cada destino que o guarda. Um clique num instantâneo restaura-o, e um clique num sítio dentro de um instantâneo restaura a partir exatamente dali.

- **Destinos** nomeia todos os lugares com backups desta entrada e quantos. Os lugares externos só são lidos quando carregas em **Verificação**, porque listar armazenamento na nuvem demora um pouco. Um lugar para onde a entrada já não faz backup aparece como **Só cópias anteriores** e ainda se pode restaurar a partir dele.
- Cada instantâneo tem um menu: **Adicionar etiqueta** ou **Alterar etiqueta**, **Comparar com outro backup**, que lista os ficheiros que vieram, saíram ou mudaram, **Exportação** como tar, zip ou um repositório restic próprio, e eliminar num só lugar.
- Os **Despejos da base de dados** de um container ficam numa lista própria, com transferência, gravação numa pasta e importação.

Quantos instantâneos ficam é definido pelo destino, vê [Durante quanto tempo guardar](offsite-recovery.md#keep).

![O separador Backups de um container: uma barra por dia e os destinos que guardam cada instantâneo.](assets/screenshots/backups/entry-backups-dark.png#only-dark)
![O separador Backups de um container: uma barra por dia e os destinos que guardam cada instantâneo.](assets/screenshots/backups/entry-backups-light.png#only-light)

### Separador Definições {#settings-tab}

Toda a entrada começa com **Agendamento e destino**:

- **No agendamento** põe a entrada dentro ou fora das execuções agendadas.
- **Plano próprio** dá a esta entrada um plano só dela, vê [Um plano próprio para uma entrada](schedules.md#own-plan). Sem ele, corre no plano da sua secção ou no backup completo.
- Uma fila de botões, **Local** e um por destino externo, diz para onde vão os backups. **Segue o padrão** significa que a entrada vai para onde vai a sua secção, e um clique nele abre **Destinos por secção**. Vê [Localização por entrada](offsite-recovery.md#placement).
- **Repositório** nomeia onde os backups são escritos (**Guardado em**). Esse lugar fica fixo desde o primeiro backup, as cópias podem mudar a qualquer altura.

Os cartões abaixo dependem do tipo de entrada:

| Tipo | Cartões |
|---|---|
| Container | **Pastas a copiar** (que pastas mapeadas são copiadas, com **Ignorar pastas de cache (CACHEDIR.TAG)**), **Padrões de exclusão**, **Parar outros contentores**, **Comandos antes e depois**, **Base de dados**, **Quando a app está ocupada**, **Atualizar após um backup bem-sucedido**, **Instantâneo em vez de parar**, **Nomes anteriores** |
| VM | **VM** com **Método** (**Ordenado (encerramento)** ou **Instantâneo a quente**) e **Só blocos alterados**, vê o [guia de backup de VMs](vm-backup-ssh-setup.md#per-vm-method) |
| Conjunto de pastas | **Pasta** e **Padrões de exclusão** |
| Unidade flash | **Exportação zip do flash**, que escreve cada backup do flash como um `.zip` simples para uma pasta à tua escolha, num só passo, para que uma ferramenta de sincronização nunca veja meio ficheiro. Um backup do flash com menos do que devia ter mantém o último ZIP no lugar, e a execução da exportação avisa disso. |
| Conjunto de dados ZFS | os conjuntos de dados filhos, os containers a parar e os comandos à volta do instantâneo, **Réplica** e **Testar o acesso aos instantâneos**, vê [Conjuntos de dados ZFS](zfs-datasets.md) |

As definições guardam-se sozinhas à medida que as mudas.

![O separador Definições de um container, com Plano próprio no primeiro cartão.](assets/screenshots/backups/entry-settings-dark.png#only-dark)
![O separador Definições de um container, com Plano próprio no primeiro cartão.](assets/screenshots/backups/entry-settings-light.png#only-light)

### Separador Verificação {#verify-tab}

- **Verificações** mostra a verificação de restauro que se segue ao primeiro backup. **Verificar restauro** volta a executá-la no backup mais recente: o BombVault restaura uma amostra de até 100 ficheiros e 256 MiB para uma pasta temporária e pede ao restic que leia cada ficheiro de volta e o compare com os seus hashes.
- **Teste de arranque**, num container, restaura o backup mais recente numa cópia isolada e arranca-a: um nome que começa por `bombvault-test-`, uma rede Docker interna própria sem portas publicadas e sem caminho para a LAN, 1 CPU e 2 GiB de memória. Passa quando a cópia se declara saudável, responde na primeira porta exposta ou, não havendo nenhuma das duas, continua a correr. O original nunca é tocado, e a cópia, a sua rede e os seus dados são removidos depois, também após um reinício a meio de um teste. Os containers na rede do host, os privilegiados, os com dispositivos e os que precisam de outro container aparecem como não testáveis. **Teste de arranque** em **Definições, Verificação, Verificações automáticas** testa um container por execução, o testado há mais tempo primeiro.
- **Anomalias** define a sensibilidade e o nível de notificação próprios desta entrada e mostra os seus achados em aberto.
- **Tamanho por pasta** mostra o que ocupa espaço no backup mais recente e quanto dele trouxe o último backup, um nível de cada vez.
- **Execuções recentes** lista as últimas execuções com o início, o fim e a duração. Um clique mostra os detalhes.

![O separador Verificação de um container com Tamanho por pasta aberto.](assets/screenshots/backups/entry-verify-dark.png#only-dark)
![O separador Verificação de um container com Tamanho por pasta aberto.](assets/screenshots/backups/entry-verify-light.png#only-light)

## Padrões de exclusão {#excludes}

O cartão **Padrões de exclusão** aceita um padrão por linha. Escreve os caminhos como os vês dentro do container, por exemplo `/config/Library/Application Support/Plex Media Server/Cache`, ou um nome simples como `.git`, que corresponde a qualquer profundidade. Num conjunto de pastas, escreve o caminho como o anfitrião o conhece, por exemplo `/mnt/user/data/cache`. Uma pré-visualização por baixo do campo mostra a que resolve cada linha, e avisa quando uma linha não corresponde a nada ou excluiria tudo. Uma linha com `/` ou `*`, ou uma que nomeie uma pasta inteira copiada, é ignorada, por isso uma exclusão nunca pode esvaziar um backup. Para deixar uma pasta inteira de fora, desmarca-a em **Pastas a copiar**.

Para imagens conhecidas como Plex, Jellyfin, Sonarr, Radarr, Immich e Nextcloud, o cartão oferece as pastas que a aplicação volta a encher sozinha, como caches, registos e miniaturas. Nada é excluído até carregares em **Excluir seleção**.

Um novo conjunto de pastas começa com o grupo **Lixo do sistema** ligado: `.DS_Store`, `Thumbs.db`, `desktop.ini`, `@eaDir` (Synology), `.Recycle.Bin` (o Unraid Recycle Bin plugin), `.Trash-*` e `*.tmp`. Cada linha é um nome simples, por isso corresponde a qualquer profundidade dentro da pasta, e podes desligar qualquer uma antes de criar o conjunto. Um conjunto de pastas existente mantém as exclusões que tem. **Excluir lixo do sistema** na sua janela de edição oferece as mesmas linhas, e elas só entram no conjunto quando carregas em **Excluir seleção** e guardas.

O mesmo cartão traz avisos que uma análise de pastas não vê. O Immich, por exemplo, guarda os seus álbuns, rostos e datas numa base de dados PostgreSQL num container à parte, por isso um backup de ficheiros só do container do Immich restaura as fotografias sem nada disso, e o restauro parece ter corrido bem. Faz também backup do container da base de dados.

## Despejos da base de dados {#database-dumps}

Os containers PostgreSQL, MySQL e MariaDB que o BombVault reconhece são despejados antes de cada backup, a partir do servidor em execução, para um ponto de restauro próprio junto dos ficheiros. Isso cobre as imagens oficiais, PostGIS, TimescaleDB, pgvector, pgautoupgrade, as imagens de base de dados do Immich, as imagens MariaDB da linuxserver, yobasystems e jc21 e o mysql-server da Oracle. Um container que apenas parece uma base de dados recebe a mesma opção, desligada até a escolheres.

O despejo vai direto para o repositório e nunca é escrito num disco. As credenciais vêm das variáveis do próprio container, segredos `*_FILE` incluídos, e nunca saem dele. O cartão **Base de dados** no separador Definições do container diz se a pasta de dados é guardada com o container parado, copiada em execução, ou não guardada, e liga ou desliga o despejo para este container. A etiqueta `bombvault.dbdump=false` faz o mesmo e prevalece sobre o cartão. **Despejos automáticos das bases de dados** em **Definições, Geral** liga-o ou desliga-o para todos.

Um despejo falhado não faz o backup falhar. Aparece como uma execução falhada própria, com o motivo, e envia uma notificação. Os despejos nunca são carregados de volta sozinhos: transfere um, simples ou comprimido, grava-o numa pasta, ou importa-o para uma base de dados acabada de arrancar com um clique no separador **Backups**. Para recuperar um despejo sem o BombVault, vê [Recuperar um despejo da base de dados](offsite-recovery.md#database-dumps). Quando um falha, vê [Um despejo da base de dados falhou](troubleshooting.md#a-database-dump-failed).

## Comandos e outros containers {#commands}

- Os **Comandos antes e depois** correm dentro do container com `sh -c`, por exemplo para gravar uma cache em disco. Um comando falhado antes do backup aborta-o. Um comando antes do backup que termine com o código 75 adia um backup agendado, vê abaixo.
- **Parar outros contentores** nomeia containers a parar enquanto este é copiado, como a sua base de dados. Depois arrancam outra vez pela ordem do `depends_on` do Compose, e por predefinição cada um tem de se declarar saudável antes de o seguinte arrancar. A espera e o seu limite estão em **Reiniciar após o backup** no cartão **Ordem** em **Definições, Agendamento**.
- Um container que partilha o espaço de nomes de rede, PID ou IPC de outro que é parado, através de `--net=container:` ou do `network_mode: service:` do Compose, para e arranca com ele mesmo que não esteja na lista, e arranca depois do container a que se junta. Um container reiniciado recebe uma rede nova, e um que ficasse a correr na rede antiga teria só o seu loopback. A lista em si fica como a definiste. Se um container destes não pôde ser parado, a execução diz que ele fica sem rede até ser reiniciado. O mesmo vale para o container do próprio BombVault, que nunca se para a si mesmo e não pode ser posto na lista.
- Um container que não volta a correr depois é indicado na execução e numa notificação, também quando o próprio backup falhou.

## Quando a aplicação está ocupada {#app-busy}

Um backup pára o seu container, por isso um backup agendado pode esperar enquanto alguém usa a aplicação e tentar de novo sozinho. O cartão **Quando a app está ocupada** no separador Definições de um container configura-o:

- A **Verificação de atividade** pergunta ao Plex, Jellyfin ou Emby, antes de um backup agendado, se alguém está a ver. O Plex é consultado com o token do seu próprio `Preferences.xml`, por isso não há nada a introduzir. O Jellyfin e o Emby pedem uma **Chave de API**, criada no painel do servidor em API keys.
- **Ignorar sessões em pausa após (min)** deixa um filme em pausa durante a noite deixar de contar.
- **Fazer backup quando o servidor não responde** decide o que acontece quando não se consegue perguntar ao servidor: ligado, o backup corre e o registo di-lo. Desligado, espera como se alguém estivesse a ver.
- **Esperar enquanto o contentor envia dados** serve para aplicações sem essa verificação, como um servidor de jogos ou um programa de transferências. O backup espera enquanto o container enviou mais do que **Ocupado a partir de (KiB/s)** em qualquer leitura nos últimos **Calmo durante (min)** minutos.
- **Tentar de novo a cada (min)**, **Tentar só dentro de um intervalo horário** com **Tentar entre**, e **Esperar no máximo (horas)** definem como correm as tentativas.
- **No fim da espera** ou desiste, o que mostra o backup em **Precisa de atenção** com **Tentar de novo**, ou **Fazer backup na mesma**.

Um backup em espera fica registado como **Adiado**, a âmbar, e o resto da execução continua. A visão geral, a lista e a entrada mostram-no com a próxima tentativa, e o alerta de atraso e o estado de proteção esperam por ele até ao prazo. Recebes uma notificação quando é adiado e outra quando termina, não uma por tentativa. Uma tentativa espera enquanto outro backup corre, e uma espera continua depois de um reinício. Um backup que inicias tu corre sempre logo, seja com **Fazer backup agora**, **Tentar de novo**, uma seleção, a API ou o MCP, e um que passa termina uma espera que estivesse pendente.

O separador Definições do container tem também uma via mais antiga e simples: **Esperar até a app estar inativa** com **Esperar no máximo (horas)**. O backup começa assim que a aplicação está inativa, o que o cartão **Inatividade antes do backup** em **Definições, Agendamento** define. Os membros de uma stack Compose que vencem na mesma execução esperam juntos, e o backup em espera não segura nenhum bloqueio, por isso os outros containers avançam.

![O cartão Quando a app está ocupada de um container Plex com a verificação de atividade configurada.](assets/screenshots/backups/app-busy-dark.png#only-dark)
![O cartão Quando a app está ocupada de um container Plex com a verificação de atividade configurada.](assets/screenshots/backups/app-busy-light.png#only-light)

### Exemplo: o Plex espera enquanto alguém está a ver {#example-plex}

O backup completo corre todas as noites às 03:00. Ninguém deve ver um filme parar porque o Plex foi abaixo para o backup, mas o backup deve na mesma acontecer antes da manhã.

1. Abre **Backups**, depois a entrada `plex`, depois o separador **Definições**.
2. Em **Quando a app está ocupada**, põe a **Verificação de atividade** no Plex. Não há nada a escrever.
3. Põe **Ignorar sessões em pausa após (min)** em `30`, para que um filme em pausa à meia-noite deixe de contar à meia-noite e meia.
4. Deixa **Fazer backup quando o servidor não responde** ligado.
5. Põe **Tentar de novo a cada (min)** em `15`.
6. Liga **Tentar só dentro de um intervalo horário** e põe **Tentar entre** em `22:00` e `06:00`.
7. Põe **Esperar no máximo (horas)** em `3` e **No fim da espera** em **Fazer backup na mesma**.

O que acontece numa noite em que alguém ainda está a ver às 03:00: o BombVault pergunta ao Plex, encontra uma sessão a reproduzir e regista o backup como **Adiado**, com a próxima tentativa às 03:15. Os outros containers da execução são copiados como de costume. Quando o filme acaba, às 04:10, a tentativa das 04:15 encontra o Plex inativo e faz o backup, e a espera termina com uma notificação. Se o filme ainda estivesse a correr às 06:00, a espera chegaria ao fim e o Plex seria copiado na mesma, por isso os seus backups nunca ficam mais de três horas para trás.

Para uma verificação tua, faz o **Comando pré-backup** terminar com o código 75 enquanto a aplicação está ocupada, por exemplo `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Há mais sintomas e soluções em [Resolução de problemas](troubleshooting.md#deferred-backups).

## Exportações simples {#exports}

O restic continua a ser o motor, mas uma exportação dá-te uma cópia que não precisa nem do BombVault nem do restic.

- **Exportar container** e **Exportar VM** em **Mais** na página de uma entrada escrevem uma cópia tal como está agora, junto ao repositório: as pastas copiadas do container mais o seu template Unraid, ou as imagens de disco da VM mais a sua definição, restauráveis com `virsh define`.
- **Exportação** num instantâneo escreve um backup guardado na pasta de exportação do servidor, como tar, zip ou um repositório restic próprio, e pode levar um instantâneo ou todos. Um backup do flash e um auto-backup contêm chaves, por isso exportar um deles precisa de uma palavra-passe de início de sessão e de um início de sessão na instância.

As exportações são ficheiros simples por predefinição. **Cifrar exportações com age** em **Definições, Sistema** sela-as, o ZIP do flash incluído, para os destinatários que listares, cada um uma chave pública age ou uma chave pública SSH. Descifra-las mais tarde fora do servidor com a chave privada correspondente. Com a cifragem ligada e sem nenhum destinatário válido, uma exportação falha com um erro claro em vez de alguma vez escrever ficheiros simples.

## O que uma execução te diz {#run-notes}

- **Tempos.** Cada execução mostra início, fim e duração, e cada entrada tem as suas **Execuções recentes**.
- **Porque é que um backup foi lento.** Enquanto um backup corre, o BombVault vigia quão ocupados estão o CPU, os discos e a rede. Quando um backup demora muito mais do que o habitual e uma coisa estava claramente no limite, a execução di-lo, por exemplo "O disco de destino disk1 estava 98% ocupado". Caso contrário não diz nada.
- **Progresso das verificações e das podas.** O registo de atividade e o cartão **Verificar os backups** mostram até onde o restic contou, por exemplo *12 de 47 packs*, com o tempo que falta nesse passo quando há dados que cheguem para o estimar.
- **Ficheiros que não conseguiu ler.** Um backup que correu mas deixou alguma coisa de fora torna a sua linha laranja e nomeia o que ficou de fora.
