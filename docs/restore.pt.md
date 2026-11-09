# Restauração

Toda a restauração começa na página **Restauração**: escolhe uma secção, procura o que queres de volta e escolhe um backup. Uma só janela restaura todos os tipos de entrada e pergunta por passos. Por predefinição nada muda no original, porque o backup volta para ao lado dele.

![A página Restauração com as secções como mosaicos, a pesquisa e o cartão Stacks.](assets/screenshots/restore/page-dark.png#only-dark)
![A página Restauração com as secções como mosaicos, a pesquisa e o cartão Stacks.](assets/screenshots/restore/page-light.png#only-light)

## A página Restauração {#page}

- **Secções** lista as secções como mosaicos, cada um com quantas entradas têm instantâneos.
- **O que queres trazer de volta?** pesquisa as entradas da secção escolhida. Cada linha mostra o seu instantâneo mais recente, e **Abrir entrada** leva à página da entrada.
- **Encontrar um ficheiro** procura um ficheiro ou uma pasta pelo nome em todos os backups, vê [Encontrar um ficheiro](#find-a-file).
- **Stacks** lista os projetos Compose com pelo menos dois containers copiados, por exemplo `immich` com a sua base de dados e o Redis. A pesquisa encontra uma stack pelo nome ou por qualquer um dos seus containers.
- **A decorrer** mostra uma restauração enquanto corre, também depois de fechares a janela.
- **Emergência** inicia o assistente de emergência para um servidor que desapareceu ou foi configurado de novo, vê [Assistente de emergência](offsite-recovery.md#emergency).
- **Outras origens** guarda **De uma pasta de backups antiga**, para os backups de outro BombVault, **Importar do plugin Appdata.Backup** e **Tarefas do Duplicati ou do Duplicacy**.

![A página Restauração num telemóvel.](assets/screenshots/restore/page-phone-dark.png#only-dark)
![A página Restauração num telemóvel.](assets/screenshots/restore/page-phone-light.png#only-light)

## A janela de restauração {#window}

Um clique numa entrada abre **Restaurar** com o seu nome. Pergunta por passos, e cada passo pode ser mudado antes de começares:

1. **Backup**: o instantâneo recomendado vem pré-selecionado. É o mais recente, ou, depois de uma anomalia em aberto, o último antes dela. O backup feito antes de uma restauração anterior guarda o estado que essa restauração substituiu, por isso é ignorado, a não ser que não haja outro. Os mais antigos estão na lista, e **Comparar** mostra o que difere entre dois deles.
2. **Origem**: de onde vêm os ficheiros, quando um instantâneo está em vários lugares. **Local** é o mais rápido.
3. **O quê**: **Todos**, ou **Ficheiros individuais**, onde marcas os ficheiros e as pastas que queres.
4. **Para onde**: o destino. As opções dependem do tipo de entrada:

    | Destino | O que faz |
    |---|---|
    | **Ao lado do original** | A predefinição. Um container volta como um segundo container chamado `<nome>-restored` com portas próprias, os discos e a NVRAM de uma VM ficam numa pasta chamada `<pasta>-restored`, e um conjunto de pastas volta como `<caminho>-restored`. Nada muda no original. |
    | **Substituir o original** | Sobrescreve o que lá está agora. Com **Fazer antes um backup do estado atual**, que vem ligado por predefinição, o BombVault copia primeiro o estado de hoje para poderes voltar atrás. Se esse backup falhar, a restauração não começa. Isto também funciona para um container ou uma VM que já não está no servidor: as pastas por cima das quais a restauração escreve têm um backup próprio. |
    | **Para uma pasta** | Só os ficheiros, para os veres. |
    | **Descarregar como ZIP** | A unidade flash, para uma pen nova. O BombVault nunca escreve na unidade flash em uso. |
    | **Num novo conjunto de dados** e **Restaurar dentro do conjunto de dados** | Conjuntos de dados ZFS, vê [Conjuntos de dados ZFS](zfs-datasets.md#restore). |

5. **O que acontece** diz por palavras o que a restauração vai fazer, e uma cena mostra o destino que escolheste.

Antes de **Restaurar** se desbloquear, a janela verifica que o repositório responde, que a chave o abre, que o instantâneo existe e que o destino tem espaço. Enquanto uma verificação falha o botão fica bloqueado, e o (i) nele diz qual.

A janela mostra também o que a restauração muda em comparação com o que lá está agora: ficheiros novos, substituídos e inalterados, com a lista a pedido, e os ficheiros do destino que não estão no backup e ficam onde estão. O restic calcula isto como um ensaio a partir do tamanho e da data de modificação, sem ler os ficheiros, e uma árvore muito grande pára aos 30 segundos e di-lo. Para containers e VMs compara as definições que a restauração recria com as que estão em execução: imagem e etiqueta, portas, nomes de variáveis e volumes, ou memória, vCPUs, discos e rede.

Algumas coisas podem parar uma restauração antes de mudar o que for. O IP estático e as portas publicadas de um container têm de estar livres, e a sua rede tem de existir. No Unraid, cada restauração no lugar recusa uma pasta que não esteja num pool ou partilha montados, porque os ficheiros ficariam na memória e desapareceriam depois de um reinício. Uma restauração no lugar nomeia todos os outros containers cujo bind mount chega a uma pasta onde ela escreve, por exemplo "este caminho também é usado por nextcloud-db", mas isso só avisa.

Algumas opções aparecem onde se aplicam: **Deixar parado depois**, para veres um container ou uma VM antes de voltar a correr, e **Restaurar sem GPU e runtime** num host que não tem a GPU ou o runtime com que o backup foi feito.

A restauração corre em segundo plano. Podes fechar a janela, e o progresso aparece também em **A decorrer**. Uma restauração longa mostra uma percentagem e pode ser cancelada, e uma restauração cancelada fica registada como cancelada, não como falhada. Uma restauração cancelada enquanto o container está a parar, ou uma que não consegue remover o container, ainda não restaurou nada, por isso o container volta a arrancar. Um container que recebe os volumes de outro com `--volumes-from` mantém-nos, e a restauração recusa começar enquanto esse outro container não existir.

![A janela de restauração no passo O quê, com ficheiros individuais marcados.](assets/screenshots/restore/window-what-dark.png#only-dark)
![A janela de restauração no passo O quê, com ficheiros individuais marcados.](assets/screenshots/restore/window-what-light.png#only-light)

### Para onde vai uma restauração para uma pasta {#folders}

**Pastas de restauro** em **Definições, Restauro** define a **Pasta de restauro predefinida**, com uma **Subpasta por secção** se quiseres, e uma **Exceção** para uma secção. Os ficheiros escolhidos ficam diretamente na pasta de destino: um ficheiro volta como só esse ficheiro, uma pasta como a pasta com o seu conteúdo, e escolhas de pastas diferentes guardam só o caminho abaixo da pasta que partilham. O BombVault recusa um destino que não esteja em armazenamento montado, porque no Unraid uma pasta dessas vive na memória e desaparece num reinício, e nunca restaura para a sua própria pasta de dados.

## Exemplo: recuperar um ficheiro {#example-single-file}

Alguém editou o `config.php` do Nextcloud ontem à noite e o Nextcloud já não arranca. O backup noturno das 03:00 ainda tem o ficheiro bom. Queres esse ficheiro, não o container inteiro.

1. Abre **Restauração**, escolhe o mosaico **Containers** e escreve `nextcloud` em **O que queres trazer de volta?**.
2. Clica em `nextcloud`. A janela abre com o backup das 03:00 pré-selecionado como o recomendado.
3. Deixa a **Origem** em **Local**.
4. Em **O quê**, escolhe **Ficheiros individuais**. Abre as pastas até `config` e marca `config.php`.
5. Em **Para onde**, escolhe **Para uma pasta** e escolhe uma pasta, por exemplo `user/restore`, que é `/mnt/user/restore` no host.
6. Lê **O que acontece**: o ficheiro fica na pasta de destino, e nada muda no container. Carrega em **Restaurar**.

A janela termina com **Concluído: os ficheiros estão em /mnt/user/restore**. Encontras o ficheiro como `/mnt/user/restore/config.php`, sem as pastas por cima. Compara-o com o estragado, copia-o de volta para a pasta appdata e reinicia o Nextcloud.

Para pôr o ficheiro diretamente de volta, escolhe **Substituir o original** no passo 5. Mantém **Fazer antes um backup do estado atual** ligado, para o BombVault copiar o estado estragado antes de escrever, escreve `nextcloud` para confirmar e carrega em **Restaurar**. O Nextcloud é parado enquanto o ficheiro escolhido é escrito de volta no seu lugar e volta a arrancar depois, e só os ficheiros que marcaste são escritos. Uma restauração que falha a meio deixa o Nextcloud parado, porque o ficheiro que ele lê pode estar meio escrito, e a execução di-lo.

![A janela de restauração em O que acontece, com Substituir o original escolhido.](assets/screenshots/restore/window-replace-dark.png#only-dark)
![A janela de restauração em O que acontece, com Substituir o original escolhido.](assets/screenshots/restore/window-replace-light.png#only-light)

## Encontrar um ficheiro {#find-a-file}

Quando sabes o nome de um ficheiro, mas não sabes que entrada ou que backup ainda o tem, escreve o nome em **Encontrar um ficheiro** e carrega em **Pesquisar**. O texto simples corresponde em qualquer parte do nome e ignora maiúsculas e minúsculas, por isso `invoice` encontra `Invoice-2024.pdf`. Com `*` ou `?`, o padrão tem de corresponder ao nome inteiro, por isso `*.pdf` encontra todos os PDF. Escreve só o nome, sem pasta.

A pesquisa lê todos os backups dos containers, dos conjuntos de pastas, dos conjuntos de dados ZFS e da unidade flash. Cada resultado mostra a entrada, o caminho e todos os backups que têm o ficheiro, do mais recente para o mais antigo, com a data, o tamanho e o destino onde foi lido. Um backup em que o ficheiro tem o mesmo tamanho e a mesma hora de modificação que no anterior diz **sem alterações**, para veres onde o ficheiro mudou pela última vez. Um clique num backup abre nele a janela de restauração, com **Ficheiros individuais** escolhido e o ficheiro marcado. Um ficheiro da unidade flash volta como parte do ZIP da unidade flash, porque a unidade flash só é restaurada por inteiro.

A pesquisa demora um pouco, porque o restic percorre todos os backups. **Parar** termina-a, e sair da página também. Por baixo do campo aparece que destinos foram lidos:

- Os destinos neste servidor e na tua rede são sempre pesquisados.
- Um backup que está em vários destinos só é lido uma vez, no primeiro. Um destino cujos backups foram todos lidos noutro sítio diz **já pesquisado noutro destino**.
- Os destinos externos, como S3, B2 ou SFTP, ficam de fora, a não ser que ligues **Pesquisar também nos destinos externos**. Aí só são lidos os backups que nenhum destino aqui tem, normalmente os mais antigos que uma retenção mais longa mantém lá. A primeira pesquisa num destino destes pode demorar bastante, e alguns fornecedores cobram pelos dados que são transferidos.
- Um destino que não foi possível abrir diz **não foi possível ler**, e o (i) ao lado explica porquê. Os outros destinos continuam a ser pesquisados.

As máquinas virtuais são guardadas como discos inteiros, por isso os ficheiros dentro delas não são pesquisados. O auto-backup e os backups de outro BombVault em **Outras origens** também não são pesquisados. Uma pesquisa lista os primeiros 200 ficheiros, por isso um nome curto como `log` pode precisar de mais algumas letras.

## O que volta, por tipo {#kinds}

Os **containers** são recriados a partir da sua definição guardada através da API do Docker, com o seu entrypoint, hostname, definições de paragem, volumes e montagens, limites de recursos, driver de registo, definições de DNS e ligações antigas. Um container volta na imagem com que corria quando o backup foi feito, não numa mais recente transferida depois, e uma imagem em falta é transferida primeiro, com o início de sessão de **Registries de containers** em **Definições, Restauro**. A sua GPU ou runtime (`--gpus`, `--runtime=nvidia`) volta também. Num host sem esse driver ou runtime, a restauração di-lo e oferece **Restaurar sem GPU e runtime**. Uma ligação a um container que falta, ou que está parado quando o restaurado arranca, é deixada de fora, e o histórico da execução di-lo. Uma restauração que falha depois de o container ter sido removido volta a pô-lo, parado.

Um backup pode conter pastas que o container já não tem selecionadas, por exemplo um mais antigo feito antes de uma pasta desaparecer. Uma pasta que desapareceu ou está vazia volta. Uma que agora tem ficheiros fica como está, porque podes tê-la desmarcado de propósito, e a verificação antes da restauração e o histórico da execução indicam-na. Recupera-a com **Ficheiros individuais** ou **Para uma pasta**.

As **VMs** têm o seu XML definido outra vez por SSH e os seus discos e a NVRAM UEFI voltam a ser ligados, mesmo depois de a VM ter sido eliminada. Vê [Backups de VMs](vm-backup-ssh-setup.md) para VMs com instantâneos do Unraid e discos em zvols.

Os **conjuntos de pastas** voltam com a sua propriedade, modos e datas como o restic os guarda. Quando o destino só recusa a propriedade, como faz o `/mnt/user`, a execução termina com uma nota que o diz. O BombVault compara então os ficheiros com o backup, e um ficheiro que não chegou faz a restauração falhar.

A **unidade flash** nunca toca no `/boot` em uso. A restauração é uma transferência chamada `flash-<id>.zip`, pronta a pôr no Unraid USB Creator. O ZIP contém a `APP_KEY`, por isso transferi-lo precisa de uma palavra-passe de início de sessão e de um início de sessão na instância. A janela lista também os plugins de cada backup do flash com a sua versão e tamanho, e pode devolver um só à flash em uso: o seu ficheiro `.plg`, a sua pasta em `config/plugins` e os ficheiros de pacote que o backup tem. O Unraid instala-o no próximo arranque, ou logo em **Plugins, Install Plugin**.

Os **conjuntos de dados ZFS** têm destinos próprios e mantêm as suas propriedades ZFS, vê [Restaurar](zfs-datasets.md#restore).

O **auto-backup** só volta através do assistente de emergência, que reinicia o BombVault, para que a base de dados em uso nunca seja sobrescrita com um acesso aberto.

Os **despejos da base de dados** nunca são carregados de volta sozinhos. Uma linha em **Despejos da base de dados** pode ser transferida, gravada numa pasta ou importada para uma base de dados acabada de arrancar. Vê [Despejos da base de dados](backups.md#database-dumps).

## Restaurar várias entradas de uma vez {#several}

Na página Restauração, escolhe entradas como escolhes linhas na lista de Backups: Ctrl+clique, Shift+clique, Ctrl+A, um toque longo, ou **Selecionar**. **Restaurar** abre então uma janela para todas.

- **Que backup**: **Recomendado** toma para cada entrada o backup que a sua própria janela recomendaria. **Um momento para todas** toma para cada entrada o seu backup mais recente a partir de um **Momento** que escolhes ou anterior, por exemplo a noite antes de algo correr mal.
- **Para onde**: **Ao lado do original** ou **Substituir o original**, com **Fazer antes um backup do estado atual**.

Cada entrada é verificada como numa restauração única, e depois correm uma a seguir à outra. Cada linha mostra como correu a sua: **À espera**, **Em curso**, **Restaurado**, **Falhou** ou **Deixado de fora**. **Parar depois desta** deixa a restauração em curso terminar e não inicia mais nenhuma. A unidade flash vem como um ZIP para transferir, e os conjuntos de dados ZFS e o auto-backup são restaurados um de cada vez.

![Restaurar várias entradas: uma linha por entrada com o seu progresso.](assets/screenshots/restore/several-dark.png#only-dark)
![Restaurar várias entradas: uma linha por entrada com o seu progresso.](assets/screenshots/restore/several-light.png#only-light)

## Restaurar uma stack {#stacks}

Um clique numa stack no cartão **Stacks** abre **Restaurar stack**. Restaura todos os membros a partir do seu último backup ao mesmo tempo, e com **Iniciar por ordem de dependências após o restauro** inicia-os pela ordem do seu `depends_on`: bases de dados e caches primeiro, depois a aplicação. Quando a pasta de projeto de um membro falta no lugar que escolheste, a janela oferece o estado do Local em vez disso.

O BombVault verifica primeiro os membros e a pasta de projeto, e uma restauração que não poderia resultar não muda nada. Caso contrário, para os membros em execução, as aplicações antes das suas bases de dados, depois escreve a pasta de projeto e a seguir restaura os membros. Se a pasta de projeto não voltar, os membros são restaurados na mesma mas ficam parados, porque a pasta pode estar meio escrita e é lá que costumam estar os bind mounts de um membro. A execução de cada membro di-lo.

Uma base de dados que só tem despejos da base de dados e nenhum backup de ficheiros, e cujo container e volume com nome desapareceram, volta sem os seus dados. A verificação di-lo antes da restauração. Fica parada, e o mesmo acontece aos containers que dependem dela, para que nenhuma aplicação corra a sua configuração inicial numa base de dados vazia. Arranca-a, importa um despejo e arranca os outros. Vê [Despejos da base de dados](backups.md#database-dumps).

## Outras origens {#other-sources}

- **De uma pasta de backups antiga** abre o repositório de outro BombVault que já não corre, num disco ou num destino externo, com o seu kit de recuperação ou o `APP_KEY` escrito à mão. A sessão é só de leitura e vive apenas na memória. Vê [De uma pasta de backups antiga](offsite-recovery.md#restore-from-another-bombvault-repo).
- **Importar do plugin Appdata.Backup** lê a pasta de backups do plugin. Cada arquivo de container passa a ser um ponto de restauro do seu container, datado de quando o plugin o fez. Os arquivos já importados antes são ignorados, e os arquivos em si só são lidos. O container precisa primeiro de um backup no BombVault, para a restauração ter a sua definição. A retenção deixa em paz os pontos de restauro importados, por isso elimina um tu mesmo quando já não precisares dele.
- **Tarefas do Duplicati ou do Duplicacy** transforma as tarefas de backup dessas ferramentas em conjuntos de pastas. Só lê as configurações delas. Os backups antigos ficam com a ferramenta antiga, e o BombVault começa com um primeiro backup próprio. Nenhuma das duas ferramentas precisa de estar instalada aqui.
    - **Do Duplicati**: no Duplicati, exporta cada tarefa para um ficheiro sem palavra-passe e escolhe os ficheiros `.json`. Uma exportação que inclua as palavras-passe traz também as credenciais do destino.
    - **Do Duplicacy**: escolhe a pasta do Duplicacy Web que contém o `duplicacy.json`, ou uma pasta de que fazes backup pela linha de comandos do Duplicacy, que contém `.duplicacy`. As chaves que a linha de comandos guardou em `.duplicacy/preferences` vêm junto. O Duplicacy Web guarda as suas credenciais encriptadas, por isso a janela pede-tas.

    Nada é criado antes de veres a pré-visualização. Cada pasta de origem passa a ser um conjunto de pastas, com a pasta deste servidor onde foi encontrada, as exclusões que foi possível converter e o agendamento. Se a ferramenta antiga corria num container próprio e uma pasta não foi encontrada, és tu que a escolhes. Os backups vão para um novo destino, numa pasta `bombvault` dentro do destino antigo, seja ele uma pasta local, S3, SFTP, um remote do rclone ou o Backblaze B2 pelo seu endereço S3. As credenciais que vêm no ficheiro ficam guardadas com as tuas outras credenciais, e as que faltam são-te pedidas. As notas por baixo de cada tarefa dizem o que não veio junto, como as regras de inclusão, as expressões regulares, o tempo durante o qual os backups eram guardados e a palavra-passe de encriptação antiga, de que o BombVault não precisa.

## Depois de uma restauração {#after}

Uma cópia restaurada ao lado do original aparece em **Backups** como excluída, para não contar como tarefa em aberto. Junta-a ao agendamento se a quiseres manter. Um container ou uma VM que estava a correr quando foi copiado volta a correr, um que estava parado continua parado. No Unraid, um container restaurado reaparece no separador Docker como uma aplicação normal e editável, e uma VM no Gestor de VMs com os seus discos e a NVRAM ligados.
