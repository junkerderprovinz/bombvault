# Externo e recuperação

!!! note "As cópias externas esperam depois de uma reconstrução"
    Quando o Assistente de emergência ou **Descobrir backups** reconstrói entradas sem as definições antigas, as cópias externas dessas secções ficam em pausa até a predefinição delas ser confirmada. Consulta [Depois de uma reconstrução](#after-a-rebuild).

Os backups locais protegem-te de um container perdido ou de uma má atualização. As cópias externas e um kit de recuperação testado protegem-te da máquina inteira, de ransomware ou de um incêndio. Esta página cobre os destinos e as cópias externas, tornar uma cópia à prova de adulteração, provar que consegues restaurar, emparelhar vários servidores e recuperar tudo quando o próprio BombVault desaparece.

## Cópias externas {#off-site-copies}

Mantém o backup local rápido e junta um ou mais destinos fora de casa. O BombVault copia para lá os novos backups com `restic copy` numa base de melhor esforço, por isso um percalço externo nunca faz o backup local falhar. Nesta forma o repo local continua primário e o repo externo é uma cópia, mas o repo primário de uma secção não tem de ser local de todo; consulta [Repositórios primários remotos](#remote-primary-repositories) mais abaixo para fazer backup diretamente para S3, um rest-server e afins em vez de copiar para lá.

- **Vários destinos por secção.** Cada secção (containers, VMs, flash, Auto-backup, pastas e conjuntos de dados ZFS) pode copiar para vários destinos externos ao mesmo tempo, para poderes manter, por exemplo, um rest-server em casa de um amigo e um bucket S3 em paralelo. O cartão **Destinos por secção** na página Destinos liga ou desliga cada linha de uma secção para um destino.
- **Quando as cópias correm.** O cartão **Copiar** de um destino define **Quando copiar**: **Depois de cada backup**, ou segundo um plano próprio que reúne os backups e copia a horas fixas. Em **Definições, Agendamento, Cópias externas por secção**, **Exceção** dá a uma secção o seu próprio plano de cópia. **Replicar agora** no cartão **Cópia externa** de uma secção em **Definições, Destinos** inicia uma cópia a pedido.
- **A retenção** define-se por destino em **Durante quanto tempo guardar**, para poderes manter as cópias externas mais tempo como arquivo. A cópia no campo externo de uma secção segue o cartão **Retenção externa** em **Definições, Destinos**; deixa-o todo a zero para nunca podar automaticamente os backups externos.
- **Os limites de largura de banda** (**Largura de banda externa** em **Definições, Destinos**, ou o cartão **Copiar** de um destino) limitam a taxa de envio/receção do restic para que as cópias não saturem a tua WAN.
- Enquanto uma cópia corre, o cabeçalho de estado da visão geral diz **Cópia externa em curso**. É um indicador ativo, não uma barra de percentagem, porque o `restic copy` não expõe progresso legível por máquina.

!!! note "Restaurar de qualquer local"
    Cada entrada lista os seus backups como uma única linha cronológica por todos os locais onde um backup está, no separador **Backups**. Um backup copiado para o B2 aparece uma vez, marcado com cada local que o tem. Um restauro usa o primeiro local a que consegue chegar, começando pelo repositório para onde a entrada é escrita, e podes escolher outro local por backup. Os destinos externos só são lidos quando clicas em **Verificar**, porque listar o armazenamento na nuvem demora. Eliminar num local verifica primeiro os outros e diz se era a última cópia.

## Destinos {#destinations}

Um destino é um lugar para onde vão os backups: uma pasta local, uma partilha, um servidor teu ou um serviço de nuvem. Configura-se uma vez e depois serve todas as secções. Abre **Destinos** a partir do respetivo mosaico de definições, ou liga lá **Mostrar também na barra lateral** para o fixar.

**Adicionar destino** abre uma janela em quatro passos:

1. **Fornecedor.** Cada serviço está listado com o seu logótipo, com uma pesquisa por cima, em grupos: serviços de armazenamento com buckets S3 (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 e outros), o teu próprio servidor S3 (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), o teu próprio servidor e partilhas (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, um caminho montado), armazenamento na nuvem (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud e o resto que o rclone suporta) e os servidores recetores do teu grupo. Cada um diz quão bem serve para backups: as unidades na nuvem abrandam com muitos pedidos, por isso o primeiro backup e a poda demoram mais aí.
2. **Iniciar sessão.** Os campos dependem do serviço: uma chave de acesso para S3, utilizador e palavra-passe para WebDAV e SMB, uma palavra-passe de aplicação onde o início de sessão de dois fatores bloqueia a normal, a chave SSH pública do BombVault para SFTP e a Storage Box, ou um token para serviços que iniciam sessão por um navegador. Para esses, a janela mostra um comando `rclone authorize` para correr num computador com navegador; o token que imprime vai para o campo. **Testar** verifica o início de sessão antes de se guardar o que quer que seja. A janela lista depois as pastas no destino, com **Nova pasta** para criar uma e o espaço livre onde o serviço o indica. Uma pasta vazia é o mais seguro; o BombVault põe lá uma pasta por secção.
3. **Proteção.** A janela diz claramente o que o serviço consegue. Um rest-server em modo append-only recusa a eliminação, e o teste de adulteração verifica-o. Um bucket S3 pode manter versões antigas com versionamento e bloqueio de objetos, o que o BombVault ainda não consegue verificar. Uma unidade na nuvem não consegue recusar a eliminação de todo: quem entra no servidor entra também nessa cópia. Liga **Proteção contra eliminação (append-only)** apenas onde o outro lado recusa mesmo a eliminação; o BombVault nunca poda aí. **Para uma emergência** diz o que o kit de recuperação guarda para este destino.
4. **Para que secções.** Escolhe as secções que fazem backup para aqui. A primeira cópia leva o histórico todo. As entradas que escolheram destinos próprios não recebem o novo por si, e a janela nomeia-as.

Os serviços S3 correm pelo backend S3 próprio do restic, o que permite aplicar uma classe de armazenamento e o bloqueio de objetos. Todos os outros serviços correm pelo rclone que acompanha o BombVault, e o seu remoto aparece depois no cartão **Externo (rclone)** em **Definições, Destinos**. Uma exportação de definições leva os destinos; com as credenciais incluídas leva também o início de sessão deles.

Cada destino tem uma página própria:

- **Ligação** com o seu nome, o interruptor **Disponível** (desligado significa que o BombVault deixa de escrever aqui, e as entradas que fazem backup para aqui falham com uma mensagem clara em vez de irem parar a outro sítio), a pasta, a compressão, a classe de armazenamento e **Fora das instalações**. **Alterar a ligação** testa e guarda um novo endereço ou início de sessão.
- **Credenciais**: o conjunto partilhado ou um conjunto próprio.
- **Proteção contra eliminação**, com **Testar agora a proteção contra eliminação** e quem poda esta cópia.
- **Copiar**: quando copiar, e os limites de envio e de receção.
- **Durante quanto tempo guardar**: **Curto**, **Equilibrado**, **Longo** ou **Valores próprios**, com uma linha cronológica de quais backups a regra mantém. **Perguntar antes de guardar menos** pergunta antes de uma regra mais curta apagar backups cuja única cópia está aqui.
- **Regras por secção**: o repositório, a retenção ou a compressão próprios de uma secção neste destino. **Alterar regras** abre-as, e **Repor o do destino** devolve uma secção ao destino.
- **Já está sob este destino**: secções cujo endereço escrito à mão está numa pasta deste destino. Assumir uma deixa o repositório onde está; o início de sessão, a classe de armazenamento e a proteção contra eliminação passam então a vir do destino. O BombVault verifica primeiro que o início de sessão do destino abre o repositório, e recusa pôr um repositório append-only sob um destino que não seja append-only.
- Um vaso que mostra quão cheio está o destino, com a marca onde fica cheio e quantas semanas restam ao ritmo de crescimento das últimas semanas. Um destino que não indica o seu tamanho diz **Tamanho desconhecido**.

Um servidor recetor que outra instância do teu grupo executa aparece entre os fornecedores como **Do teu grupo**; consulta [Servidor recetor](#receiving-server).

### Mover e remover {#move-remove}

- **Mover** leva a cópia de uma secção para outro destino, com todo o histórico. Tudo fica no sítio antigo; o BombVault não apaga nada aí. O movimento corre em segundo plano e pode demorar: cerca de 9 minutos por 100 GB entre discos locais, horas numa ligação lenta. A visão geral mostra-o enquanto corre.
- **Remover destino** oferece dois caminhos. **Remover destino, manter os backups** deixa de escrever lá e deixa os backups no sítio; podes lê-los mais tarde em **Restauração, De uma pasta de backups antiga**. **Remover destino e excluir os backups aí** lê primeiro o que lá está guardado e lista-o: que backups desaparecem, quais deles não existem em mais lado nenhum, e que repositórios ficam porque a proteção contra eliminação o recusa, há entradas que ainda fazem backup diretamente para eles, outro destino ou outra secção guarda lá as suas cópias, ou não foi possível lê-los. As entradas que perderiam o seu único backup são listadas à parte e mantêm-se, a menos que marques **Excluir também estas**. Confirmas escrevendo o nome do destino, e a eliminação corre em segundo plano.
- Eliminar os backups num destino exige uma palavra-passe de início de sessão e uma sessão na própria instância. Sem palavra-passe o destino ainda pode ser removido, só que os seus backups não podem ser eliminados a partir daqui.

## Localização por entrada {#placement}

O cartão **Agendamento e destino** no separador Definições de uma entrada tem uma fila de botões: **Local** e um botão por destino externo da secção. Os botões acesos recebem os backups da entrada.

- Com **Local** aceso, a entrada é escrita no repositório indicado em **Guardado em** e copiada para todos os outros destinos acesos. Apaga um destino e ele não recebe nada de novo desta entrada. Só Local não copia para lado nenhum, o que serve dados que já têm uma segunda cópia, por exemplo uma partilha que vive num NAS.
- Com **Local** apagado, a entrada é escrita diretamente no repositório direto do primeiro destino aceso e copiada daí para os outros acesos. Da primeira vez, uma caixa de diálogo cria esse repositório direto.
- Um botão fica sempre aceso, porque um backup precisa de um sítio para onde ir. Para deixar uma entrada de fora dos backups, usa **Não fazer backup**.

A localização fica fixa desde o primeiro backup da entrada, porque o BombVault nunca move backups entre repositórios; o cartão **Repositório** no separador Definições nomeia-a. As cópias podem mudar a qualquer momento. Um destino que deixa de receber uma entrada mantém as cópias que tem e apara-as à sua própria retenção na próxima execução externa da secção. Dos destinos append-only não se pode eliminar.

Por baixo da fila, o cartão diz para onde a entrada vai e o que lá está de facto: quantos sítios a guardam, quando cada destino foi visto pela última vez e se o 3-2-1 é cumprido. Um sítio é o servidor com os dados originais, cada destino externo e cada repositório marcado como **Fora das instalações**. O BombVault verifica cópias e sítios; não verifica a parte dos «dois suportes» do 3-2-1.

### Predefinições por secção {#placement-defaults}

O cartão **Destinos por secção** na página Destinos é a predefinição de cada secção. No gráfico cada linha é uma cópia, e clicar numa linha ou num ponto liga-a ou desliga-a; as linhas tracejadas estão desligadas. **Gráfico / Interruptores** mostra o mesmo como filas de botões. As cópias aplicam-se logo a todas as entradas sem escolha própria e às pastas de projeto das stacks Compose. A localização aplica-se a uma entrada nova no seu primeiro backup; alterá-la não move backups. Antes de guardar, o BombVault nomeia cada destino que ganha ou perde entradas e quantos backups isso significa.

As entradas que fazem backup para um sítio diferente da sua secção estão no cartão **Escolha própria**, cada uma com **Seguir a predefinição**. Os novos backups vão então para os destinos da secção, e os anteriores ficam onde estão.

Um novo destino externo recebe todas as entradas que não estejam em Local. A janela que o adiciona diz quantas entradas e, quando se sabe, quanto histórico isso é.

### Repositórios diretos

Desligar Local para uma entrada, para que um destino sem repositório direto passe a ser a sua casa, abre uma caixa de diálogo com uma localização sugerida junto dele, por exemplo `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, e um teste de ligação que não cria nada. **Criar e usar** cria o repositório e aponta a entrada para ele. Um repositório direto assume a chave, a classe de armazenamento, os limites, a definição append-only e a retenção do destino, e muda com eles; o cartão Repositórios mostra-o só de leitura. Quando uma chave nova do destino não o consegue abrir, o repositório direto mantém a chave que tem e o guardar di-lo. Uma entrada num repositório direto é copiada daí para os outros destinos acesos, nunca para o destino a que o repositório pertence. Os seus backups levam a etiqueta `bv:direct`, e qualquer outra passagem de retenção mantém-nos, de modo que um repositório direto que perdeu a ligação ao seu destino nunca envelhece pelas regras locais. O B2 alcança-se pelo seu ponto de acesso S3, com o ID da chave e a chave da aplicação introduzidos como credenciais S3; uma chave limitada à pasta do próprio destino não chega à pasta ao lado, por isso limita antes a chave à pasta acima.

### Fora das instalações

Um destino ou um repositório com nome pode ser marcado como **Fora das instalações**. Os remotos começam marcados; desliga-o para um rest-server no mesmo edifício. A marca apenas conta sítios e o 3-2-1 nos cartões. Não altera nenhuma cópia.

Uma réplica ZFS é uma cópia fora das instalações, mas não um backup com histórico. Mantém apenas os instantâneos que definires, e as versões antigas e os ficheiros avulsos continuam a vir dos backups. Vai de um servidor ZFS diretamente para o outro, por SSH para um servidor ZFS ou para o ponto de receção de uma instância emparelhada. O grupo de emparelhamento só transporta o pedido. Consulta [Réplica](zfs-datasets.md#replica).

### Depois de uma reconstrução {#after-a-rebuild}

As escolhas de cópia vivem nas definições do próprio BombVault. Depois de uma reconstrução por **Descobrir backups** ou pelo Assistente de emergência sem definições restauradas, desaparecem, e copiar tudo mandaria outra vez para o B2 as entradas que tinhas deixado de fora. As cópias externas de cada secção reconstruída ficam por isso em pausa. A visão geral di-lo, e o cartão **Destinos por secção** oferece **Confirmar padrão** com uma pré-visualização do que a próxima execução copia e os nomes nos backups que não têm entrada, que podes deixar de fora aí. Só a confirmação termina a pausa; importar um ficheiro de definições devolve regras e predefinições mas não a termina.

## Repositórios primários remotos {#remote-primary-repositories}

O caminho próprio de uma secção não se limita a uma pasta local: aponta-o diretamente para um remoto restic (`s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo`, `rclone:remote:bucket/path`) e o BombVault faz backup diretamente para lá, sem cópia local separada e sem passo de cópia. Esta é uma forma diferente das cópias externas acima: aí o repo local é primário e o repo externo é um arquivo de melhor esforço dele; aqui o repo remoto **é** o primário, e é a única cópia a menos que copies também essa secção para outro destino.

O caminho de cada secção (Containers, VMs, Flash, Auto-backup, Pastas, Conjuntos de dados ZFS) define-se na página do destino **Pasta local**: **Alterar** junto à secção no seu cartão **Ligação** abre uma janela com um interruptor **Local / Remoto**:

- **Local** mostra o conhecido seletor de pastas.
- **Remoto** troca-o por um campo de URL simples e pelas definições de ligação que um destino externo usa, configuradas desta vez para este primário. Daí obténs:
    - **Um teste de ligação** contra o caminho real, antes de contares com ele.
    - **Limites de largura de banda** (envio/receção) para que um backup agendado para um primário remoto não sature a tua WAN: as mesmas flags `--limit-upload`/`--limit-download` do restic que as cópias externas usam, aplicadas ao próprio backup.
    - **Proteção append-only**, verificada com o mesmo teste de adulteração ativo (uma sonda DELETE real contra o outro lado) que os destinos externos recebem. Com ela ligada, o BombVault recusa podar o próprio repo, já que não há uma cópia local separada por trás e as credenciais nesta máquina não devem poder apagar a única cópia do backup.
    - **Um alarme de orçamento de crescimento**, amostrado a partir da mesma tendência do tamanho do repo que o cartão Armazenamento já acompanha.

Nada disto é obrigatório: um caminho remoto escrito à mão e sem definições de segurança guardadas faz backup sem limites, pode ser podado e não tem alarme de orçamento. As definições de segurança servem para quando queres as proteções que uma cópia externa recebe, sem um destino separado só para as ter.

!!! note "As credenciais de nuvem e REST são partilhadas"
    Um primário remoto autentica-se com as credenciais de nuvem partilhadas em **Definições, Destinos**; não há um armazém de credenciais separado para os repos primários.

### SMB e WebDAV sem montagem no host {#smb-webdav}

O cartão **Externo (rclone)** em **Definições, Destinos** tem um formulário para uma partilha Windows ou Samba e para um servidor WebDAV (Nextcloud, ownCloud, SharePoint ou outro). Preenche um nome curto, o anfitrião e a partilha (SMB) ou o URL e o tipo de servidor (WebDAV), o utilizador e a palavra-passe, e o BombVault escreve a secção rclone por ti. O rclone ofusca a palavra-passe ele próprio antes de a guardar; adicionar um com um nome que já existe substitui essa secção em vez de juntar uma segunda. **Adicionar destino** oferece SMB e WebDAV como fornecedores com os mesmos campos.

O formulário responde com a localização pronta, por exemplo `rclone:nas:backups`. Usa-a como caminho de uma secção ou como destino e junta uma subpasta se quiseres (`rclone:nas:backups/bombvault`). A partilha é o primeiro segmento do caminho, não faz parte do nome.

Este é o melhor caminho do que montar a partilha no Unraid: o restic desaconselha manter um repositório numa partilha CIFS montada, e aqui nada é montado. O NFS não está no formulário porque nem o restic nem o rclone têm um backend NFS; para NFS, monta a exportação no host e usa-a como pasta local.

## Proteção contra eliminação (append-only)

Liga a proteção contra eliminação num destino externo para que o ransomware, ou um host comprometido, não possa apagar nem reescrever os teus backups. O outro lado (um `restic/rest-server` a correr em modo `--append-only`) **impõe-na**. O BombVault apenas a **verifica** e nunca mostra verde só por uma afirmação de configuração. Uma pasta neste servidor não consegue recusar a eliminação, porque quem controla o BombVault controla também a pasta, por isso os destinos locais não oferecem o interruptor.

O caminho mais rápido para uma cópia append-only é um BombVault emparelhado com um [servidor recetor](#receiving-server): ele executa o rest-server por ti, em modo append-only.

!!! note "Uma eliminação bem-sucedida em `/locks/` é esperada"
    Append-only não significa que nada possa alguma vez ser removido. O restic tem de tomar e libertar os seus próprios bloqueios, por isso `/locks/` continua, por conceção, gravável e eliminável. Os instantâneos e os dados por trás deles, que é o que o ransomware procuraria, não podem ser removidos. Se testares tu próprio o outro lado, uma eliminação que tenha êxito em `/locks/` é o comportamento correto e não um buraco na proteção.

!!! warning "Os repos append-only nunca são podados a partir desta máquina"
    Um destino append-only nunca poda daqui os backups antigos; podar é trabalho do outro lado. Define um **alarme de orçamento de crescimento** para ele, para seres avisado antes de o tamanho do repo disparar.

## Teste de adulteração

O BombVault prova periodicamente a garantia append-only tentando de facto uma eliminação contra o repo externo, dirigida a um objeto inexistente. **Testar agora a proteção contra eliminação** na página de um destino corre-o a pedido, e **Verificar proteção contra eliminação** em **Definições, Verificação, Verificações automáticas** corre-o num agendamento.

- **Recusada** significa protegido.
- **Aceite** significa sem proteção.
- Um resultado **inconclusivo** (servidor inacessível, erro de autenticação) nunca inverte o veredicto guardado.

Uma verdadeira passagem de protegido para desprotegido dispara um único alerta.

## Ensaios de DR

O BombVault oferece dois níveis de prova de que os teus backups são realmente restauráveis, e não apenas existentes.

- **Ensaios de verificação de restauro (local).** O BombVault corre periodicamente `restic check --read-data-subset` (limitado, nunca um restauro completo que encha o disco). O ritmo fica em **Definições, Verificação, Verificações automáticas**, e o cartão **Verificar os backups** no mesmo mosaico mostra o resultado.
- **Restauro de teste (externo).** O BombVault restaura um alvo real do repo externo para uma sandbox isolada, verifica-o ficheiro a ficheiro e byte a byte, e depois limpa. Isto prova que consegues recuperar a partir do externo, e não apenas que o repo responde.

O cartão **Estado de proteção** na visão geral resume isto por secção em quatro verificações: um backup atual neste servidor, uma cópia externa, proteção contra eliminação aí e um restauro de teste a partir de lá. Um clique numa secção mostra os detalhes, e o cartão só fica verde com factos verificados.

## Emparelhamento de instâncias {#pairing}

Vários BombVaults trabalham juntos como um só grupo: mostram o estado de proteção uns dos outros, assumem funções uns para os outros e podem guardar a chave uns dos outros. Uma instância entra no grupo com doze palavras.

Na primeira instância abre **Definições, Emparelhamento** e prime **Gerar frase**. Três cartões de passos mostram como decorre. Doze palavras aparecem numa janela com um botão **Copiar**. Em cada outra instância abre o mesmo sítio, prime **Introduzir frase** e cola ou escreve-as, ou prime **Colar** nessa janela. Uma palavra que não está na lista é nomeada com a sua posição enquanto escreves, e a última palavra leva uma soma de controlo, por isso uma palavra mal escrita ou trocada é apanhada antes de se emparelhar o que quer que seja. Gera a frase numa só instância: duas instâncias que a geram ambas formam dois grupos separados. Se ninguém aparecer ao fim de um minuto, a página oferece duas saídas: mostrar as palavras outra vez para as introduzir lá, ou introduzir as palavras da outra instância e entrar no grupo dela num só passo. O emparelhamento funciona sem palavra-passe de início de sessão, mas define uma: sem ela, qualquer pessoa que consiga abrir a interface web pode ler as palavras e, através do grupo, pedir funções que entregam a palavra-passe restic das instâncias nele. O cartão de emparelhamento di-lo até haver uma palavra-passe definida. Com palavra-passe, mostrar a frase de novo pede-a. **Sair do grupo** tira uma instância de lá outra vez.

Quem conhece as palavras pode entrar no grupo, por isso trata-as como uma palavra-passe.

**Como os membros se alcançam.** Cada instância aprende o seu próprio endereço na rede a partir do teu navegador no momento em que inicias sessão; fica em **Endereços conhecidos** na página de emparelhamento, juntamente com os que a instância encontrou sozinha; corrige-o aí se houver um proxy inverso ou uma porta invulgar pela frente. Na mesma rede os membros anunciam esse endereço por multicast e falam diretamente, e onde o multicast não consegue atravessar uma rede de containers, como a bridge predefinida do Docker, uma instância procura em vez disso a sua própria sub-rede pelos outros com uma chamada assinada a que só um membro do grupo pode responder, pelo que o emparelhamento acaba em segundos sem relay. Se nada aparecer, **Não consegues encontrá-la?** aceita um endereço à mão, para outra sub-rede ou uma porta não padrão. As instâncias em redes diferentes passam por um relay, escolhido na mesma página; cada uma das quatro formas vem com uma imagem:

- **Relay do projeto** (a predefinição): `parleyport.halleluja.design`, o relay que o KnightLoader também usa. Nada a configurar.
- **Relay próprio**: o container [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) das Unraid Community Apps (também em **Definições, Aplicações**), ou uma das tuas instâncias que já seja acessível do exterior com **Servir como relay** ligado. Essa instância responde então em `/relay/connect` no seu próprio endereço, atrás do proxy inverso e do certificado que já tem, e deixa entrar só o teu grupo. Introduz o endereço do relay em cada instância que o deva usar.
- **Sem relay**: os membros encontram-se automaticamente na mesma rede, e em mais lado nenhum.

**O que o relay vê.** Cada chamada entre membros é selada com AES-256-GCM sob uma chave derivada das doze palavras, e essa chave nunca sai das tuas instâncias. O relay fica a saber um hash que agrupa as ligações, para que instância é uma mensagem, o seu tamanho e quando passa. Uma chamada direta na rede local é selada da mesma forma e também assinada, pelo que nada depende do certificado autoassinado que uma instância apresenta. As chamadas que entregam algo (respostas a funções, uma palavra-passe restic, inícios de sessão de recetores, depósitos de chaves) são também assinadas com uma chave do membro que as envia, fixada à primeira vista; consulta [Modelo de segurança](configuration.md#security-model).

**O que viaja pelo grupo.** O estado de proteção na página Instâncias, um pedido para verificar uma secção agora, pedidos de função e as respostas, ofertas de destino, e o que um recetor ou recolhedor precisa quando a sua função é permitida: as localizações de repositório da outra instância e a sua palavra-passe restic. Os dados dos backups nunca; esses continuam a ir diretamente para os backends do restic. A APP_KEY também não: a palavra-passe restic abre os repositórios dessa instância e mais nada, nem os seus segredos guardados, sessões ou códigos de recuperação.

**Entradas de antes do emparelhamento.** As instâncias adicionadas com um token de frota, e os repositórios recebidos e as origens de recolha configurados com a APP_KEY da outra instância, ficam depois da atualização e são marcados com **Emparelhar novamente**. Continuam a funcionar: no primeiro arranque o BombVault substitui cada APP_KEY guardada pela palavra-passe restic derivada dela. Emparelha as duas instâncias, depois edita a entrada e escolhe a respetiva instância. Até lá, essas entradas ficam no cartão **Sem instância** na página Instâncias.

O único sítio que ainda aceita uma APP_KEY à mão é [De uma pasta de backups antiga](#restore-from-another-bombvault-repo), para o caso de a outra instância ter desaparecido e não poder responder num grupo.

## Instâncias e funções {#roles}

A página **Instâncias** (um mosaico de definições que pode ser fixado à barra lateral) mostra o grupo como uma rede de quem faz o quê por quem, um cartão para esta instância e um cartão para cada outro membro, os telemóveis Android e os servidores ZFS. **Emparelhar uma instância** flutua em baixo à direita. As partes que estão ligadas definem-se em **Definições, Instâncias, Vários servidores**: **Instâncias**, **Recetor** e **Recolha**.

Cada cartão mostra o estado de proteção do membro e as suas funções como mosaicos. Uma função é algo que uma instância faz por outra:

- **Recetor** guarda as cópias externas deste servidor num rest-server em modo append-only. Este servidor envia depois de cada backup e não consegue apagar nada lá, mesmo que alguém o tome.
- **Recolhedor** recolhe os backups deste servidor segundo o seu próprio agendamento. Este servidor não precisa de estar acordado para isso.
- **Servidor ZFS** aceita as réplicas ZFS deste servidor. Cada entrada ZFS escolhe-o em **Réplica** nas suas definições.
- **Guardião da chave** guarda uma cópia selada do kit de recuperação deste servidor. Consulta [Guardião da chave](#key-keeper).

Ligar uma função envia um pedido, e nada começa até alguém na outra instância premir **Permitir**. O pedido aparece no cartão dessa instância como distintivo, e os seus **Detalhes** listam-no com **Permitir** e **Recusar**; uma função permitida pode ser revogada aí a qualquer momento. Uma linha tracejada na rede é uma função ainda à espera. Uma instância no BombVault 9.x não consegue responder a um pedido, por isso uma função fica aí ativa assim que a ligas, e um pedido de uma instância 9.x espera aqui por **Permitir**.

**Detalhes** num cartão abre o resto: os repositórios recebidos e as origens de recolha desse membro, ofertas de destino, monitorização, **Remover** e **Esquecer a chave**.

## Recetor e recolhedor {#receiver}

![Uma cópia externa recebida nos detalhes de uma instância, vigiada só para leitura, com uma verificação de integridade corrida neste hardware.](assets/screenshots/receiver.png)

*Uma cópia externa recebida nos detalhes de uma instância, vigiada só para leitura, com uma verificação de integridade corrida neste hardware.*

Na máquina que **recebe** cópias append-only de outro BombVault, os repositórios recebidos dão-te monitorização independente, só de leitura, no hardware recetor, para que uma falha silenciosa no outro extremo não passe despercebida. Liga **Recetor** em **Definições, Instâncias**, depois abre os **Detalhes** da instância emissora na página Instâncias e adiciona o repositório recebido (só de leitura, aberto com a palavra-passe restic da instância emissora, que ela recebe através do [grupo de emparelhamento](#pairing)). Obténs:

- **Um inventário por origem**, para veres exatamente que containers, VMs e conjuntos de pastas chegaram.
- **Último recebido** por origem, para saberes quão fresca é cada uma.
- **Um `restic check` independente** corrido no hardware recetor, para que a integridade seja verificada onde os dados realmente estão, não só no emissor.
- **Um interruptor de homem-morto:** um alerta quando uma origem deixa de enviar dentro de uma janela que defines.
- **Alertas de integridade:** um alerta quando uma verificação do lado recetor falha.

O recetor é estritamente só de leitura. Nunca escreve no repositório recebido, por isso nunca pode quebrar a garantia append-only em que o emissor confia.

Um recolhedor funciona ao contrário: com **Recolha** ligada, uma instância lê os backups de outro membro para o seu próprio repositório segundo o seu próprio agendamento. As suas origens ficam também nos detalhes desse membro.

### Servidor recetor {#receiving-server}

A máquina recetora também pode executar o rest-server para onde as outras copiam. Configura-se da primeira vez que permites um pedido de **Recetor** que o peça: a janela diz que o servidor precisa de um servidor recetor e pede uma pasta numa partilha, com **Nova pasta** para criar uma, e uma porta (8000, a menos que outro container a use). O BombVault depois:

1. recusa se já existir um container chamado `rest-server` ou se outro container tiver a porta;
2. obtém `restic/rest-server` e arranca-o através do socket do Docker em modo append-only, com repositórios privados e um ficheiro de início de sessão na pasta;
3. escreve o respetivo template Unraid na flash, para que o container continue editável no separador Docker, ou oferece o template como transferência quando a flash está fora de alcance;
4. executa o teste de adulteração contra ele e mostra se recusa eliminações.

As instâncias do teu grupo encontram depois o servidor entre os fornecedores em **Do teu grupo**, com o nome da máquina recetora, e a função **Recetor** oferece-o como a escolha segura. Cada instância recebe um início de sessão próprio da primeira vez que usa o servidor, e escreve lá apenas na sua própria pasta. Com **Recetor** ligado em **Definições, Instâncias**, **Servidor recetor** no cartão desta instância volta a abrir o servidor. A sua janela lista os inícios de sessão em **Parceiros com início de sessão**, e **Revogar início de sessão** retira um; o que essa instância já copiou fica na pasta. A configuração cria também um **Início de sessão fora do grupo**, cuja palavra-passe a janela mostra uma só vez.

Uma instância que chega à máquina recetora apenas através do relay não pode usar o servidor, porque o relay não transporta backups. Adiciona primeiro o endereço da máquina recetora na página de emparelhamento. Quando o BombVault corre num endereço IP próprio (no br0, por exemplo), preenche **Endereço para parceiros**, porque o servidor escuta no endereço do host.

## Exemplo completo: duas máquinas Unraid, de ponta a ponta

Tudo o que está acima descreve as peças. Este é um conjunto completo, porque as peças são mais fáceis de montar quando as viste montadas uma vez.

Duas máquinas: **TOWER** corre os containers e envia os backups; **VAULT** recebe-os e impõe append-only. Substitui pelos teus nomes, endereços e caminhos de partilha.

**1. Emparelha as duas máquinas.** No TOWER abre **Definições, Emparelhamento**, prime **Gerar frase** e introduz as doze palavras no VAULT em **Introduzir frase**. Define primeiro uma palavra-passe de início de sessão nas duas.

**2. No TOWER, pede ao VAULT que seja o recetor.** Na página Instâncias, liga **Recetor** no cartão do VAULT, escolhe as secções e **o servidor recetor do VAULT** como local, e envia o pedido.

**3. No VAULT, permite-o.** A página Instâncias do VAULT mostra o pedido no cartão do TOWER. Prime **Permitir**. Da primeira vez, o VAULT pede uma pasta para o seu servidor recetor, como `user/appdata/rest-server`, arranca o rest-server em modo append-only e corre o teste de adulteração contra ele.

**4. De volta ao TOWER.** A partir daí o TOWER copia para o VAULT depois de cada backup, e o novo destino aparece na página Destinos do TOWER com a sua proteção contra eliminação.

O que o teste de adulteração responde:

| Resultado | O que aconteceu |
| --- | --- |
| **protegido** | O VAULT recusou a eliminação. Este é o único estado que passa. |
| **sem proteção** | O VAULT aceitou uma eliminação. `--append-only` falta ou foi removido. |
| **inconclusivo** | Nenhum dos dois. Normalmente o URL não é o que o próprio restic usa, ou as credenciais mudaram. Nada é registado e nenhum alerta dispara. |

**5. Vê o que chega, se quiseres.** Liga **Recetor** em **Definições, Instâncias** no VAULT, abre os **Detalhes** do TOWER e adiciona o repositório recebido. O VAULT recebe a palavra-passe restic do TOWER pelo grupo; ninguém escreve uma chave.

**6. Torna-o mútuo.** Repete os passos no sentido contrário: o VAULT pede ao TOWER que seja o seu recetor. Cada máquina impõe então append-only para a outra, e nenhuma pode apagar os backups da outra.

Um rest-server que corras à mão também funciona: adiciona-o com **Adicionar destino** como rest-server, com o utilizador e a palavra-passe do seu `.htpasswd`, e liga a proteção contra eliminação. Mantém `--append-only` nas suas opções; sem isso o outro lado volta a ser uma partilha normal.

!!! warning "Um caminho local é um caminho dentro do container"
    Onde o BombVault pede uma pasta neste servidor, introduz-a relativa à montagem do host, como `user/appdata/rest-server/bombvault-containers`, e não `/mnt/user/appdata/...`. O BombVault corre num container, onde o `/mnt` do host está montado noutro lado; um caminho absoluto do host não existe lá dentro. Se colares um, o BombVault diz-te o caminho relativo a usar em vez dele.

## Assistente de emergência {#emergency}

O **Assistente de emergência** traz de volta um servidor que desapareceu ou foi instalado de novo. Começa na página **Restauração** em **Emergência**, e a partir de **Restaurar a partir de um backup** na página de boas-vindas de uma instalação nova. Corre em quatro passos: **Acesso**, **Definições**, **Seleção** e **Restauro**.

1. **Acesso.** Mostra ao BombVault a chave dos backups antigos, de uma de três formas:
    - **Carregar kit de recuperação**: o ficheiro que descarregaste durante a configuração. Contém a chave e cada destino, e o BombVault preenche o resto a partir dele. Um kit selado com age pede a identidade para a qual foi selado; é usada uma vez e não é guardada.
    - **Através do grupo**: uma instância do teu grupo guarda a chave como depósito. Precisas das doze palavras do grupo, do teu código de desbloqueio e de alguém nessa instância. Consulta [Guardião da chave](#key-keeper).
    - **Escrever a chave**: a `APP_KEY` do servidor antigo e onde estão os seus backups.

    O BombVault experimenta então a chave em cada destino e diz para cada um se é legível, não encontrado, bloqueado com outra chave, inacessível ou ainda sem início de sessão. Ainda nada foi guardado, e a chave fica na memória por um tempo limitado. **Adotar a chave** faz este servidor escrever daqui em diante os seus backups com essa chave; **Anular a adoção** volta à chave própria até o primeiro backup ter corrido. Um servidor que já tem backups próprios recusa a adoção e aponta antes para [De uma pasta de backups antiga](#restore-from-another-bombvault-repo). **Guardar credenciais** guarda os inícios de sessão que o kit traz para destinos remotos, e pergunta antes de substituir os inícios de sessão que este servidor já tem.
2. **Definições.** **Trazer de volta as definições** restaura as definições do próprio BombVault a partir do Auto-backup mais recente: destinos, credenciais, agendamentos e regras. O BombVault reinicia para as aplicar, através do socket do Docker, para que a base de dados de definições em uso nunca seja sobrescrita com um handle aberto, e o assistente continua depois por si. O início de sessão deste servidor mantém-se: continuas a entrar com a palavra-passe que usas agora. Os segredos que o servidor antigo tinha guardado e que não podem ser assumidos são listados, para os poderes introduzir de novo.
3. **Seleção.** O BombVault percorre os teus backups e lista os containers, as VMs, os conjuntos de pastas e os conjuntos de dados ZFS que encontra. O backup mais recente vem pré-selecionado em todo o lado. O que este servidor ainda tem começa desligado, porque um restauro o substituiria. Os conjuntos de pastas já não sabem de onde vieram depois de uma reconstrução, por isso vão para uma pasta que escolhes.
4. **Restauro.** Tudo o que foi selecionado volta de uma vez, deixado parado se quiseres arrancar cada container e VM tu próprio, o que é o mais seguro depois de uma perda total. Depois descarrega um kit de recuperação novo: o antigo pertence à instalação anterior.

!!! tip "Migração planeada versus desastre"
    O Assistente de emergência restaura as definições do próprio BombVault a partir de um backup. Para uma mudança *planeada* para uma máquina nova, podes em vez disso levar a tua configuração diretamente com o cartão **Exportar / importar configurações** (um ficheiro JSON portátil). Consulta [Configuração](configuration.md#portable-settings-export-and-import).

### De uma pasta de backups antiga {#restore-from-another-bombvault-repo}

**De uma pasta de backups antiga** em **Outras origens** na página Restauração abre o repositório de uma instância BombVault *diferente* (uma partilha montada em `/mnt`, ou um endereço remoto) com o kit de recuperação dessa instância ou a sua `APP_KEY` escrita, numa sessão única, só de leitura. Navega pelos containers, VMs, conjuntos de pastas e conjuntos de dados ZFS guardados lá, escolhe um backup e restaura-o, e o objeto restaurado passa a ser um container, VM ou conjunto de pastas local normal. Nunca se escreve nada no outro repositório, e as tuas próprias definições de backup ficam intocadas (a sessão vive na memória e expira sozinha). Mover um container do servidor A para o servidor B não significa reapontar as tuas definições e revertê-las depois. Esta janela é de uma só vez: abre uma sessão, restaura o que escolheres e esquece a outra instância. Se preferires uma disposição permanente, em que esta máquina recolhe os backups de outra instância para o seu próprio repositório num agendamento, essa é a função **Recolhedor** na página Instâncias.

Um container cuja rede não existe neste servidor, como uma rede Unraid `br0` num host Docker simples, mostra um seletor de rede por baixo da sua linha. O BombVault cria-o na rede que escolheres, juntamente com as suas outras redes. O IP fixo e o endereço MAC pertenciam à rede antiga e são descartados, por isso a nova rede atribui-os.

## Guardião da chave {#key-keeper}

Um guardião da chave é uma instância do teu grupo que guarda uma cópia selada do kit de recuperação deste servidor. Não consegue abrir a cópia por si. Se este servidor alguma vez se perder, uma nova instalação que se junte ao teu grupo pede a cópia ao guardião e abre-a com o teu código de desbloqueio.

1. Na página Instâncias, liga **Guardião da chave** no cartão da instância que o deve guardar, e escolhe **Fazer depósito**. Fazer um depósito exige uma palavra-passe de início de sessão.
2. Este servidor mostra o **Código de desbloqueio** uma só vez: vinte letras e dígitos em quatro grupos de cinco. Escreve-o e confirma com **Anotei o código**. Este servidor não guarda o código, apenas o que precisa para selar um depósito para ele.
3. O guardião tem agora o depósito, e este servidor renova-o quando o kit muda. **Depósito da chave** no cartão desta instância lista o **Depósito deste servidor** com cada guardião e o seu estado, e os **Depósitos que este servidor guarda** para os outros. **Criar um novo código** substitui o código; cada guardião passa para o novo quando recebe o novo depósito.

Nesse dia precisas de três coisas: as doze palavras do teu grupo, o código de desbloqueio e acesso à interface do guardião, onde alguém tem de premir **Permitir**. No Assistente de emergência escolhe **Através do grupo**, escolhe o depósito, pede-o e escreve o código de desbloqueio. O código é usado no servidor novo e não é enviado ao guardião. No guardião, o pedido aparece em **Depósito da chave** com quem pede, se essa instância já foi vista aí alguma vez e quantas vezes pediu; **Permitir** entrega a cópia selada, e permitir exige uma palavra-passe de início de sessão. A instalação nova tem 30 minutos para a recolher depois do Permitir, um pedido a que ninguém responde expira ao fim de um dia, e uma instância que pede demasiadas vezes tem de esperar uma hora.

Sem o código ninguém consegue abrir a cópia, o guardião incluído. Se o código se perder, cria um novo enquanto este servidor ainda corre; se o servidor já desapareceu, o kit de recuperação e a APP_KEY escrita ainda funcionam. O guardião pode apagar um depósito a qualquer momento, e o dono tem então de pedir de novo.

## Kit de recuperação {#recovery-kit}

Esta é a peça que torna possível a recuperação de desastres mesmo quando não há nenhum BombVault em execução.

**Descarregar kit de recuperação** no cartão **Para emergências** em **Definições, Restauro** dá-te a **chave**, a **palavra-passe restic derivada**, cada **destino** com o repositório de cada secção, os inícios de sessão de que uma instalação nova precisa para lhes chegar, e os **comandos restic exatos**, para poderes restaurar diretamente com a CLI do restic em qualquer máquina. Exige uma palavra-passe de início de sessão. O kit imprime a chave também com a encriptação desligada, porque as definições de containers e VMs são sempre seladas com ela. Após uma adoção, imprime a chave dos backups adotada e a APP_KEY deste servidor, de que uma instalação nova só precisa para trazer as definições de volta. Um cartão **Guarda o teu kit de recuperação** na visão geral lembra-te até o teres guardado.

!!! danger "Guarda o kit de recuperação fora do servidor"
    O kit contém o segredo que desencripta os teus backups. Guarda-o num local seguro e separado do servidor (um gestor de palavras-passe, uma cópia impressa num cofre). Se perderes o BombVault e a `APP_KEY` sem kit de recuperação e sem depósito da chave, os teus backups encriptados não podem ser recuperados.

!!! warning "O instantâneo mais recente nem sempre é o que deves restaurar"
    Desde o restic 0.17, `restic snapshots` mostra o tamanho de cada instantâneo. Após uma perda de dados o instantâneo mais recente pode ser o esvaziado, por isso não restaures um instantâneo muito mais pequeno do que os anteriores. Após ransomware pode ser o encriptado, com o tamanho habitual. Se o BombVault ainda corre, verifica primeiro a sua página **Anomalias**: ela indica o último backup bom, e a janela de restauro pré-seleciona-o. Um restauro não precisa de nenhum dado de anomalias do BombVault, e a pausa da retenção só mantém sempre mais instantâneos.

### Selar o kit

Se ligaste **Cifrar exportações com age** em **Definições, Sistema**, o kit também é selado com ela e é transferido como `bombvault-recovery-kit.md.age`. Está em ASCII-armor em vez de binário, por isso continua a ser texto simples: colá-lo num gestor de palavras-passe ou imprimi-lo funciona exatamente como antes, só que o conteúdo é ilegível sem a tua chave.

!!! warning "Não guardes a chave age dentro do kit"
    Precisas da tua chave age **privada** para abrir um kit selado. Guarda-a num sítio que não dependa do próprio kit, ou terás duas coisas para recuperar em vez de uma. Selar compensa quando o kit está guardado num sítio que não controlas totalmente (um gestor de palavras-passe partilhado, notas na nuvem, uma cópia impressa num escritório); um kit no teu próprio cofre já está protegido pelo cofre.

    Com a encriptação ligada e nenhum destinatário utilizável configurado, a transferência é simplesmente recusada. O BombVault nunca recorre a entregar a chave mestra em claro.

### Se não tiver o kit à mão

A palavra-passe não está guardada em lado nenhum, é **calculada** a partir da chave dos backups. Com a chave e uma shell podes reproduzi-la tu próprio:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

É um HMAC-SHA256 sobre a cadeia fixa `bombvault:restic-repo`, com os bytes crus da `APP_KEY` hexadecimal como chave, impresso como 64 caracteres hexadecimais minúsculos. O mesmo valor está no kit, listado como a palavra-passe restic derivada; isto é para o dia em que o kit estiver onde tu não estás. Num servidor que adotou os backups de outro, usa a chave dos backups adotada, que é a antiga APP_KEY desse servidor.

!!! warning "Para um repositório recebido, use a chave da instância REMETENTE"
    Um repositório que chegou aqui por uma cópia externa foi criado pela máquina que o enviou, com a **sua** chave. Derivar a partir da chave da máquina recetora dá uma palavra-passe que o restic recusa, o que se lê exatamente como um repositório corrompido sem o ser. É a razão habitual para o `restic check` num repositório recebido pedir a palavra-passe vezes sem conta.

Como as definições de recuperação vivem **dentro** de cada repo (`<repo>/def`, `<repo>/vm-def`), uma pasta de repo copiada é totalmente autossuficiente, por isso o kit mais o repo é tudo o que um restauro em bare-metal precisa.

## Recuperar um dump de base de dados {#database-dumps}

Um dump de base de dados é um ponto de restauro próprio no repositório dos containers, com a etiqueta `dbdump:<container>` e um único ficheiro, `/dbdump/<container>.sql`. O BombVault lista-os, descarrega-os e importa-os no separador **Backups** do container; abaixo estão os mesmos passos só com o restic, para o dia em que o BombVault não estiver lá.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

As etiquetas `dbversion:` e `dbname:` de cada dump dizem de que versão de servidor veio e que bases contém. Um ficheiro completo termina com `-- PostgreSQL database cluster dump complete` ou `-- Dump completed`.

Importa-o para um container da mesma versão ou de uma mais recente (PostgreSQL), ou da mesma versão principal (MySQL e MariaDB), arrancado uma vez com a pasta de dados vazia para que se inicialize. O anfitrião não precisa de cliente de base de dados, o container tem um:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Para uma só base dentro de um dump completo, o MySQL e o MariaDB aceitam `--one-database <name>` no comando do cliente. Um dump de PostgreSQL tem uma secção por base, cada uma a começar numa linha `\connect <name>`: copia essa secção para um ficheiro próprio e importa-o com `-d <name>` depois de criar a base.

!!! warning "Um dump feito como root traz as contas do servidor"
    Um dump completo de MySQL ou MariaDB feito como root contém a base de sistema `mysql`, pelo que importá-lo substitui as contas do servidor novo, palavra-passe de root incluída, pelas do dump. No PostgreSQL, `role ... already exists` para o utilizador criado pelo container é esperado e inofensivo.
