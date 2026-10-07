# Externo e recuperação

!!! note "As cópias externas esperam depois de uma reconstrução"
    Quando o passo 4 reconstrói entradas sem as definições antigas, a replicação externa desses domínios entra em pausa até a localização padrão ser confirmada. Consulte [Localização por item](#placement).

Os backups locais protegem-no de um container perdido ou de uma atualização má. A replicação externa e um kit de recuperação testado protegem-no da máquina inteira, de ransomware, ou de um incêndio. Esta página cobre replicar para o externo, tornar essa cópia à prova de adulteração, provar que consegue restaurar, e recuperar quando o próprio BombVault desaparece.

## Replicação externa

Mantenha o backup local rápido e adicione uma ou mais réplicas externas. Defina um repo por domínio na página **Definições, Externo**. O BombVault replica novos instantâneos para lá com `restic copy` numa base de melhor esforço, por isso um percalço externo nunca faz o backup local falhar. Nesta forma o repo local mantém-se primário e o repo externo é uma réplica, mas o repo primário de um domínio não tem de ser local; consulte [Repositórios primários remotos](#remote-primary-repositories) mais abaixo para fazer backup diretamente para S3, rest-server, etc. em vez de replicar para lá.

- **Vários destinos externos por domínio.** Cada domínio (containers, VMs, flash, config, conjuntos de ficheiros e conjuntos de dados ZFS) pode replicar para vários destinos externos de uma só vez, não apenas um, para que possa manter, por exemplo, um rest-server na máquina de um amigo e um bucket S3 em paralelo. Adicione destinos extra em Definições, Externo, cada um com o seu próprio repositório, classe de armazenamento S3, flag append-only, retenção e orçamento de crescimento. Uma configuração externa única existente é transferida intacta como o primeiro destino, e cada destino de um domínio replica no agendamento externo desse domínio.
- **Agendamento externo por domínio** (editado ao lado de todos os outros agendamentos em Definições, Agendamentos): deixe-o em branco para replicar após cada backup local, ou defina uma cadência (por exemplo `weekly Sun 03:00`) para enviar para o externo com menos frequência do que faz backup localmente. Um botão **Replicar agora** cobre as execuções a pedido.
- **A retenção externa** vive em Definições, Retenção para que possa manter as cópias externas por mais tempo como arquivo. Deixe a política toda a zero para nunca aparar automaticamente os instantâneos externos.
- **Os limites de largura de banda** (Definições, Externo) limitam a taxa de envio/receção do restic para que a replicação não sature a sua WAN.
- Um **indicador de replicação** mostra qual o domínio que está a replicar enquanto corre (na sua página e no Painel). É um indicador ativo, não uma barra de percentagem, porque o `restic copy` não expõe nenhum progresso legível por máquina.

!!! note "Restaurar de qualquer local"
    Cada container, VM, conjunto de ficheiros, a flash e a configuração da aplicação listam os seus backups como uma única linha do tempo por todos os locais onde um backup se encontra. Um backup copiado para o B2 aparece uma vez, marcado com cada local que o guarda. Um restauro usa o primeiro local a que consegue chegar, começando pelo repositório onde o item é escrito, e pode escolher outro local por linha. Os locais externos só são lidos quando os abre. Eliminar num local verifica primeiro os outros e diz se era a última cópia.

## Destinos {#destinations}

Definições, Externo começa com **Destinos**: os locais para onde vão as cópias externas, configurados uma só vez para todos os domínios. Um destino aparece depois como um botão na linha **Localização** de cada domínio e item. Na primeira vez que é ligado para um domínio, o BombVault cria o repositório desse domínio numa pasta dentro dele, por exemplo `rclone:onedrive:BombVault/containers`. Flash, Auto-backup e conjuntos de dados ZFS não têm linha **Localização**, por isso a secção externa deles oferece em vez disso **Adicionar de** e o nome do destino.

**Adicionar destino** abre um assistente em cinco passos:

1. **Para onde devem ir os backups?** Cada serviço é listado com o seu logótipo, em quatro grupos: serviços de armazenamento com buckets S3 (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 e outros), o seu próprio servidor S3 (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), o seu próprio servidor e partilhas (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, um caminho montado) e armazenamento na nuvem (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud e o resto do que o rclone suporta). Cada um indica quão adequado é para backups: as unidades na nuvem abrandam com muitos pedidos, por isso o primeiro backup e a limpeza demoram mais aí.
2. **Iniciar sessão.** Os campos dependem do serviço: uma chave de acesso para S3, um utilizador e uma palavra-passe para WebDAV e SMB, uma palavra-passe de aplicação onde a autenticação de dois fatores bloqueia a normal, a chave SSH pública do BombVault para SFTP e para a Storage Box, ou um token para os serviços que iniciam sessão através de um navegador. Para esses, o assistente mostra um comando `rclone authorize` para executar num computador com navegador; o token que ele imprime vai para o campo. **Testar ligação** verifica o início de sessão antes de se guardar qualquer coisa.
3. **Escolha uma pasta.** O assistente lista as pastas no destino, com **Nova pasta** para criar uma e o espaço livre onde o serviço o indica. Uma pasta vazia é o mais seguro.
4. **Proteção contra eliminação.** O assistente diz com clareza o que o serviço consegue fazer. Um rest-server em modo append-only recusa eliminações, e o teste de adulteração verifica-o. Um bucket S3 pode manter versões antigas com versionamento e bloqueio de objetos, o que o BombVault ainda não consegue verificar. Uma unidade na nuvem não consegue recusar eliminações de todo: quem entrar no servidor entra também nessa cópia. Ligue **Imutável (append-only)** só onde o outro lado recusa mesmo a eliminação; o BombVault então nunca apara aí.
5. **Para uma emergência.** O kit de recuperação lista cada destino com o repositório de cada domínio por baixo. O início de sessão volta com o backup das definições do BombVault; numa instalação nova sem ele, configure o destino outra vez no mesmo local.

Os serviços S3 funcionam através do backend S3 do próprio restic, que é o que permite aplicar uma classe de armazenamento e o bloqueio de objetos. Todos os outros serviços funcionam através do rclone que o BombVault inclui, e o seu remote aparece então na configuração do rclone em Definições, Acesso à nuvem. Uma exportação de definições leva os destinos; com as credenciais incluídas, leva também o respetivo início de sessão.

Um servidor recetor que outra instância do seu grupo executa aparece no assistente em **Do teu grupo**; veja [Servidor recetor](#receiving-server).

O destino de um domínio criado a partir de um destino assume o nome, a localização, as credenciais, a classe de armazenamento e o interruptor imutável do destino. A sua retenção, compressão e orçamento de crescimento continuam por domínio, e a sua localização não pode mudar porque o repositório do domínio está lá. **Adicionar um destino só para este domínio** sob cada domínio continua a aceitar um URL de repositório escrito à mão.

O campo off-site de um domínio, a sua cópia principal, também pode seguir um destino. Por baixo do campo há um botão **Usar** para cada destino para o qual o domínio ainda não copia. Move a cópia principal para a pasta do domínio sob esse destino, e a cópia passa então a assumir o nome, as credenciais, a classe de armazenamento e o interruptor imutável do destino, enquanto a sua retenção continua na página Retenção. A partir daí o campo mostra a localização bloqueada. **Escrever uma localização** desbloqueia-o, e guardar uma localização escrita à mão termina a ligação.

Um destino escrito à mão cujo repositório está numa pasta de um destino pode juntar-se a ele. O destino lista esses destinos em **Já está sob este destino**, e **Assumir** pendura um deles nele. O destino mantém o seu repositório, snapshots, retenção e localização, e assume o nome, as credenciais, a classe de armazenamento e o interruptor imutável do destino. O BombVault verifica primeiro que o início de sessão do destino abre o repositório e recusa pôr um destino append-only sob um destino que não é append-only. O principal de um domínio que é assumido fica no campo off-site desse domínio.

## Localização por item {#placement}

Cada cartão de container, VM e conjunto de ficheiros tem uma linha **Localização** de botões: **Local** e um botão por destino externo do domínio, seguidos dos destinos sob os quais o domínio ainda não tem nenhum destino externo. Os botões acesos recebem os backups do item.

- Com **Local** aceso, o item é escrito no repositório mostrado em **Guardado em** e copiado para todos os outros destinos acesos. Se um destino ficar apagado, deixa de receber algo de novo deste item. Só Local não copia para lado nenhum, o que serve para dados que já têm uma segunda cópia, por exemplo uma partilha que vive num NAS.
- Com **Local** apagado, o item é escrito diretamente no repositório direto do primeiro destino aceso e copiado daí para os outros destinos acesos. Na primeira vez, um diálogo cria esse repositório direto.
- Um botão de destino cria o destino externo do domínio sob esse destino e acende-o só para este item. Todos os outros itens começam sem cópia aí.
- Um botão fica sempre aceso, porque um backup precisa de um sítio para ir. Para deixar algo fora dos backups, exclua-o.

A localização fica fixa desde o primeiro backup do item, porque o BombVault nunca move backups entre repositórios. As cópias podem mudar a qualquer momento. Um destino que deixa de receber um item mantém as cópias que tem e apara-as pela sua própria retenção na próxima execução externa do domínio; **Apagar em B2** no cartão remove-as de imediato. Quando algumas dessas cópias não existem em mais lado nenhum, a confirmação lista-as por data e pede o nome do item. De destinos append-only não se pode apagar.

Sob a linha, o cartão diz para onde vai o item e o que está lá de facto: quantos locais o guardam, quando cada destino foi visto pela última vez, e se o 3-2-1 é cumprido. Um local é o servidor com os dados originais, cada destino externo e cada repositório marcado **Fora das instalações**. O BombVault verifica cópias e locais; não verifica a parte dos «dois suportes» do 3-2-1.

### Localizações padrão

Definições, Armazenamento, **Localizações padrão** tem uma linha por domínio com os mesmos botões. As cópias aplicam-se de imediato a cada item sem escolha própria, e às pastas de projeto das stacks Compose. A localização aplica-se a um item novo no seu primeiro backup; alterá-la não move nenhum backup. Antes de guardar, a linha nomeia cada destino que ganha ou perde itens e quantos instantâneos isso significa. **Aplicar a itens sem backups** repõe no padrão todo o item que ainda não tem backup.

Um destino externo novo recebe todo o item que não está definido como Local. O diálogo que o adiciona diz quantos itens e, quando conhecido, quanto histórico isso representa, e propõe deixar de fora os itens já excluídos de outros destinos.

### Repositórios diretos

Desligar Local para um item, de modo que um destino sem repositório direto passe a ser a sua casa, abre um diálogo com uma localização sugerida junto ao destino, por exemplo `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, e um teste de ligação que não cria nada. **Criar e usar** cria o repositório e aponta o item para ele. Um repositório direto assume a chave, a classe de armazenamento, os limites, a definição append-only e a retenção do destino, e muda com eles; o cartão Repositórios mostra-o só de leitura. Quando uma chave nova do destino não consegue abri-lo, o repositório direto mantém a chave que tem e a gravação diz-o. Um item num repositório direto é copiado daí para os outros destinos acesos, nunca para o destino a que o repositório pertence. Os seus instantâneos levam a etiqueta `bv:direct`, e todas as outras passagens de retenção mantêm-nos, por isso um repositório direto que perdeu a ligação ao seu destino nunca envelhece pelas regras locais. Acede-se ao B2 através do seu endpoint S3, indicando o ID da chave e a chave de aplicação como credenciais S3; uma chave limitada à pasta do próprio destino não consegue alcançar a pasta ao lado dela, por isso limite antes a chave à pasta acima do destino.

### Fora das instalações

Um repositório nomeado pode ser marcado **Fora das instalações** no cartão Repositórios. Os repositórios remotos começam marcados; desligue isto para um rest-server no mesmo edifício. A marca só conta para locais e para o 3-2-1 nos cartões. Não muda nenhuma cópia.

### Depois de uma reconstrução

As escolhas de cópia vivem nas próprias definições do BombVault. Depois de uma reconstrução através de Descobrir backups sem um `/config` restaurado, desaparecem, e copiar tudo voltaria a enviar para o B2 os itens que tinha deixado de fora. A replicação externa de cada domínio reconstruído entra por isso em pausa. O Painel mostra-o a âmbar, e as Localizações padrão oferecem **Confirmar padrão** com uma pré-visualização do que a próxima execução copia e os nomes nos backups que não têm entrada, que pode deixar de fora ali. Só a confirmação termina a pausa; importar um ficheiro de definições traz de volta regras e padrões mas não a termina.

## Repositórios primários remotos {#remote-primary-repositories}

O caminho de cópia de um domínio (Definições, Armazenamento) não se limita a uma pasta local: aponte-o diretamente para um remoto restic (`s3:...`, `rest:http://host:8000/repo`, `sftp:utilizador@host:/repo`, `rclone:remoto:bucket/caminho`) e o BombVault copia diretamente para lá, sem cópia local separada e sem passo de replicação. É uma forma verdadeiramente diferente da replicação fora do local acima: ali o repositório local é o primário e o de fora do local é um arquivo dele na medida do possível; aqui o repositório remoto **é** o primário, e é a única cópia enquanto não configurar também uma replicação fora do local (ou um segundo remoto) para esse domínio.

Cada um dos seis campos de caminho (Containers, VMs, Flash, Auto-backup, Pastas, Conjuntos de dados ZFS) tem mesmo ao lado um interruptor **Local / Remoto**:

- **Local** mostra o explorador de pastas do costume.
- **Remoto** troca-o por um simples campo de URL, mais um botão que abre a mesma janela de teste de ligação e credenciais que os destinos fora do local usam, configurada para este primário. A partir daí obtém:
    - **Um teste de ligação** contra o caminho real, antes de depender dele.
    - **Limites de largura de banda** (envio e receção), para que uma cópia agendada para um primário remoto não sature a sua ligação WAN: os mesmos parâmetros restic `--limit-upload` e `--limit-download` que a replicação fora do local usa, aplicados à própria cópia.
    - **Proteção append-only (imutabilidade)**, verificada com o mesmo teste ativo de adulteração (uma sonda DELETE real contra o outro lado) que os destinos fora do local recebem. Com ela ligada, o BombVault recusa-se a podar o repositório: como atrás dele não há cópia local separada, as credenciais nesta máquina não podem ser capazes de apagar a única cópia da salvaguarda.
    - **Um alarme de orçamento de crescimento**, tirado da mesma tendência de tamanho do repositório que o cartão Armazenamento já acompanha.

Nada disto é obrigatório: um caminho remoto escrito à mão e sem definições de segurança guardadas copia exatamente como sempre (largura de banda ilimitada, podável, sem alarme de orçamento). A janela de segurança existe para quando quiser as mesmas proteções que uma cópia fora do local recebe, sem ter de criar um destino fora do local só para isso.

!!! note "As credenciais de nuvem e REST são partilhadas"
    Um primário remoto autentica-se com as mesmas credenciais S3/REST configuradas em Definições, Acesso à nuvem, Credenciais de nuvem partilhadas. Não há um cofre de credenciais separado para repositórios primários.

### SMB e WebDAV sem montagem no host {#smb-webdav}

Definições, Acesso à nuvem, rclone tem um formulário para uma partilha Windows ou Samba e para um servidor WebDAV (Nextcloud, ownCloud, SharePoint ou qualquer outro). Preencha um nome curto, o host e a partilha (SMB) ou o URL e o tipo de servidor (WebDAV), o utilizador e a palavra-passe, e o BombVault escreve a secção do rclone por si. O rclone ofusca a palavra-passe por conta própria antes de ser guardada; adicionar um destino com um nome que já existe substitui essa secção em vez de acrescentar uma segunda.

O formulário responde com a localização final, por exemplo `rclone:nas:backups`. Coloque-a num Caminho de backup ou num destino externo e acrescente uma subpasta se quiser (`rclone:nas:backups/bombvault`). A partilha é o primeiro segmento do caminho, não faz parte do nome.

É um caminho melhor do que montar a partilha no Unraid: o restic desaconselha manter um repositório numa partilha CIFS montada, e aqui nada é montado. O NFS não está no formulário porque nem o restic nem o rclone têm um backend NFS; para NFS, monte o export no host e aponte-lhe um Caminho de backup.

## Externo imutável (append-only)

Marque um repo externo como append-only para que ransomware, ou um host comprometido, não possa eliminar ou reescrever os seus backups. O lado remoto (um `restic/rest-server` a correr em modo `--append-only`) **impõe-no**. O BombVault apenas o **verifica** e nunca mostra verde só com base numa afirmação de configuração.

O assistente de **configuração guiada do externo** acompanha-o desde a escolha do backend (rest-server / rclone / S3), passando por um snippet de implementação de rest-server pronto a colar, um teste de ligação, o interruptor de imutabilidade (que corre o teste de adulteração de imediato) e uma estratégia de retenção, para que o externo append-only seja alcançável sem editar configs à mão.

!!! note "Uma exclusão bem-sucedida em `/locks/` é esperada"
    Append-only não significa que nada mais possa ser removido. O restic precisa criar e liberar os próprios bloqueios, por isso `/locks/` continua gravável e removível de propósito. Os snapshots e os dados por trás deles, exatamente o alvo de um ransomware, não podem ser removidos. Se você mesmo testar o lado remoto, uma exclusão bem-sucedida em `/locks/` é o comportamento correto e não uma falha.

!!! warning "Os repos imutáveis nunca são podados a partir desta máquina"
    Um externo imutável deliberadamente nunca poda instantâneos antigos. Defina um **alarme de orçamento de crescimento** para ele para ser alertado antes de o tamanho do repo descontrolar.

## Teste de adulteração

O BombVault prova periodicamente a garantia append-only tentando de facto uma eliminação contra o repo externo, dirigida a um objeto inexistente:

- **Recusada** significa protegido.
- **Aceite** significa não protegido.
- Um resultado **inconclusivo** (servidor inacessível, erro de autenticação) nunca inverte o veredicto guardado.

Uma inversão real de protegido-para-desprotegido dispara um único alerta.

## Ensaios de DR

O BombVault oferece dois níveis de prova de que os seus backups são de facto restauráveis, não apenas presentes.

- **Ensaios de verificação de restauro (local).** O BombVault corre periodicamente `restic check --read-data-subset` (limitado, nunca um restauro completo que enche o disco) e mostra um selo *Restaurabilidade verificada* por domínio. A cadência vive em Definições, Agendamentos; o selo em Definições, Integridade.
- **Ensaios de DR (externo).** O BombVault restaura um alvo real do repo externo para uma sandbox descartável, verifica-o ficheiro a ficheiro e byte a byte, e depois limpa. Isto prova que consegue recuperar do externo, não apenas que o repo responde.

O **scorecard de proteção contra ransomware** no Painel resume isto numa postura verde / âmbar / vermelha por domínio, com uma checklist com marca de idade (externo configurado, append-only verificado, replicação atual, ensaio de restauro passado, encriptação ligada, estratégia de poda definida). Cada linha vermelha liga diretamente à correção, e o cartão só fica verde com factos verificados.

## Emparelhamento de instâncias {#pairing}

Recetores, origens de recolha, a página Instâncias e o Mesh externo falam todos com outro BombVault. Fazem-no como membros de um único grupo de emparelhamento, e uma instância entra no grupo com doze palavras.

Na primeira instância, abra **Definições → Emparelhamento** e clique em **Gerar frase** nos cartões de emparelhamento. Aparecem doze palavras numa janela com um botão **Copiar**. Em cada uma das outras instâncias, abra o mesmo local, clique em **Introduzir frase** e cole-as ou escreva-as, ou clique em **Colar** nessa janela. Uma palavra que não está na lista é indicada com a sua posição logo que a escreve, e a última palavra traz uma soma de verificação, por isso uma palavra escrita incorretamente ou trocada é detetada antes de qualquer emparelhamento. Gere a frase apenas numa instância: duas instâncias que criem cada uma uma frase formam dois grupos separados. Se ninguém aparecer num minuto, o separador oferece duas saídas: mostrar as palavras outra vez para as introduzir do outro lado, ou introduzir as palavras da outra instância e juntar-se ao seu grupo num só passo. O emparelhamento funciona sem palavra-passe de acesso, mas defina uma: sem ela, quem conseguir abrir esta interface web pode ler as palavras e obter, através do grupo, a palavra-passe restic de cada instância nele. O cartão de emparelhamento avisa disso até ser definida uma palavra-passe. Com uma palavra-passe, mostrar a frase de novo pede-a. **Sair do grupo** retira uma instância outra vez.

Quem quer que conheça as palavras pode entrar no grupo, por isso trate-as como uma palavra-passe.

**Como os membros se alcançam.** Cada instância aprende o seu próprio endereço na rede a partir do seu navegador assim que inicia sessão, mostrado no cartão do relay como **Esta instância na sua rede**; corrija-o aí se houver um proxy reverso ou uma porta pouco habitual à frente. Na mesma rede, os membros anunciam esse endereço por multicast e falam diretamente entre si, e onde o multicast não consegue atravessar uma rede de contentores, como a rede bridge predefinida do Docker, uma instância passa antes a procurar na sua própria sub-rede pelas outras, com uma chamada assinada que só um membro do grupo consegue responder, por isso o emparelhamento continua a ficar concluído em segundos, sem relay. Se nada aparecer, **Não consegue encontrá-la?** por baixo do cartão de emparelhamento aceita um endereço à mão, para outra sub-rede ou uma porta não habitual. Instâncias em redes diferentes passam por um relay, escolhido no mesmo separador:

- **Relay do projeto** (a predefinição): `parleyport.halleluja.design`, o mesmo relay que o KnightLoader também usa. Nada a configurar.
- **Relay próprio**: o contentor [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) das Unraid Community Apps, ou uma das suas instâncias que já esteja acessível a partir de fora com **Servir como relay** ativado. Essa instância passa a responder em `/relay/connect` no seu próprio endereço, atrás do proxy reverso e do certificado que já tem, e deixa entrar apenas o seu grupo. Introduza o endereço do relay em cada instância que o deva usar.
- **Sem relay**: os membros encontram-se automaticamente na mesma rede, e em mais lado nenhum.

**O que o relay vê.** Cada chamada entre membros é selada com AES-256-GCM sob uma chave derivada das doze palavras, e essa chave nunca sai das suas instâncias. O relay fica a saber um hash que agrupa as ligações, para que instância é uma mensagem, qual o seu tamanho e quando passa. Uma chamada direta na rede local é selada da mesma forma e assinada também, por isso nada depende do certificado autoassinado que uma instância serve.

**O que viaja pelo grupo.** Os scorecards da página Instâncias, um pedido para verificar um domínio agora, as ofertas do Mesh para off-site, e o que um recetor ou uma origem de recolha precisa: as localizações do repositório da outra instância e a sua palavra-passe restic. Os dados de backup nunca viajam por aqui, continuam a ir diretamente para os backends restic. Nem a APP_KEY: a palavra-passe restic abre apenas os repositórios dessa instância e mais nada, nem os seus segredos guardados, sessões ou códigos de recuperação.

**Entradas anteriores ao emparelhamento.** Instâncias adicionadas com um token de fleet, e recetores e origens de recolha configurados com a APP_KEY da outra instância, mantêm-se depois da atualização e ficam marcados **Emparelhar novamente**. Os recetores e as origens de recolha continuam a funcionar: no primeiro arranque, o BombVault substitui cada APP_KEY guardada pela palavra-passe restic derivada dela. Emparelhe as duas instâncias, depois edite a entrada e escolha a sua instância. Uma instância dessas retoma o seu cartão antigo assim que surge no grupo uma instância com o mesmo nome.

O único sítio que ainda pede uma APP_KEY à mão é [Restaurar a partir de outro repo BombVault](#restore-from-another-bombvault-repo), para o caso de a outra instância ter desaparecido e já não poder responder em nenhum grupo.

## Painel recetor (o lado que recebe)

![O lado recetor, vigiado apenas para leitura, com uma verificação de integridade feita nesta máquina.](assets/screenshots/receiver.png)

*O lado recetor, vigiado apenas para leitura, com uma verificação de integridade feita nesta máquina.*

Tudo acima é o lado *emissor*. Na máquina que **recebe** cópias externas imutáveis de outro BombVault, o painel Recetor dá-lhe monitorização independente e só de leitura desses repositórios no hardware recetor, para que uma falha silenciosa no lado remoto não passe despercebida.

Ligue o interruptor **Recetor** em Definições para revelar um separador **Recetor**. Está desligado por predefinição; ative-o apenas numa máquina que de facto recebe backups externos imutáveis. Depois registe um repositório recebido (só de leitura, aberto com a palavra-passe restic da instância emissora, que a recebe através do [grupo de emparelhamento](#pairing)) para obter:

- **Um inventário de instantâneos agrupado por origem**, para que possa ver exatamente quais containers, VMs e conjuntos de ficheiros aterraram.
- **Último recebido** por origem, para que saiba quão fresco cada um é.
- **Um `restic check` independente** corrido no hardware recetor, para que a integridade seja verificada onde os dados de facto residem, não apenas no emissor.
- **Um interruptor de homem-morto:** um alerta quando uma origem deixa de enviar dentro de uma janela que definir.
- **Alertas de integridade:** um alerta quando uma verificação no lado recetor falha.

O Recetor é estritamente só de leitura. Nunca escreve no repositório recebido, por isso nunca pode quebrar a garantia append-only da qual o emissor depende.

### Servidor recetor {#receiving-server}

A máquina recetora também pode executar o rest-server para onde as outras copiam. **Configurar servidor recetor**, no topo do separador Recetor, pede uma pasta numa partilha, com **Nova pasta** para criar uma, e uma porta (8000, a menos que outro container a use). O BombVault depois:

1. recusa se já existir um container chamado `rest-server` ou se outro container ocupar a porta;
2. obtém `restic/rest-server` e arranca-o através do socket do Docker em modo append-only, com repositórios privados e um ficheiro de inícios de sessão na pasta;
3. escreve o respetivo template Unraid no flash, para que o container continue editável no separador Docker, ou oferece o template como transferência quando o flash está fora de alcance;
4. executa o teste de adulteração contra ele e mostra se recusa eliminações.

As instâncias do seu grupo encontram depois o servidor no assistente de destinos em **Do teu grupo**, com o nome da máquina recetora. Cada instância recebe um início de sessão próprio na primeira vez que escolhe o servidor e escreve lá apenas na sua própria pasta. O cartão lista esses inícios de sessão, e **Revogar início de sessão** retira um; o que essa instância já copiou fica na pasta. A configuração cria também um início de sessão para alguém fora do grupo, cuja palavra-passe o cartão mostra uma única vez.

Uma instância que chega à máquina recetora apenas através do relay não pode usar o servidor, porque o relay não transporta backups. Adicione primeiro o endereço da máquina recetora em Definições, Emparelhamento. Quando o BombVault corre num endereço IP próprio (em br0, por exemplo), preencha **Endereço para os parceiros**, porque o servidor escuta no endereço do host.

## Exemplo completo: duas máquinas Unraid, de ponta a ponta

Acima estão as peças. Isto é uma instalação completa com valores reais, porque as peças montam-se melhor depois de as termos visto montadas uma vez.

Duas máquinas: **TOWER** executa os contentores e envia as cópias, **VAULT** recebe-as e impõe a imutabilidade. Substitua pelos seus próprios nomes, endereços e caminhos de partilha.

**1. No VAULT, monte o servidor append-only.** No BombVault em TOWER vá a *Definições → Externo → Configurar*, escolha **rest-server** e gere a receita. Copie o separador **Modelo Unraid (XML)**, guarde-o no VAULT como `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, depois *Docker → Add Container* e escolha **rest-server** na lista de modelos. Antes de o iniciar, escreva a linha `htpasswd` mostrada em `/mnt/user/appdata/rest-server/.htpasswd` no VAULT. A palavra-passe de uso único é mostrada uma vez e nunca guardada: copie-a agora. Essa linha leva a mesma palavra-passe, já cifrada com bcrypt para si: o texto em claro vai nas credenciais REST em TOWER, a linha cifrada no `.htpasswd` em VAULT. Não tem de cifrar nada.

    Deixe `--append-only` no campo OPTIONS. É esse o objetivo: sem ele, o VAULT volta a ser uma partilha comum.

**2. No TOWER, aponte o repositório externo para ele.** O URL do repositório segue o padrão que a receita imprime:

    rest:http://VAULT:8000/bombvault-containers/containers

O primeiro segmento do caminho é o utilizador htpasswd, o segundo é o repositório. Introduza o utilizador e a palavra-passe gerados como credenciais REST do destino e execute o **teste de ligação**.

**3. No TOWER, ative «Imutável».** O teste de adulteração corre de imediato e tem de dizer *protegido*. O que significam as respostas:

| Resultado | O que aconteceu |
| --- | --- |
| **protegido** | O VAULT recusou a eliminação. É o único estado que passa. |
| **NÃO protegido** | O VAULT aceitou uma eliminação. Falta `--append-only` ou foi retirado. |
| **inconclusivo** | Nem uma coisa nem outra. Normalmente o URL não é o que o restic usa, ou as credenciais mudaram. Nada é registado e nenhum alerta é disparado. |

**4. No VAULT, veja o que chega.** Emparelhe as duas máquinas ([Emparelhamento de instâncias](#pairing)), ative *Definições → Geral → Recetor*, abra o separador **Recetor** e registe o repositório em apenas leitura com o TOWER como instância emissora.

!!! warning "A localização é um caminho **dentro** do contentor, escrito relativamente à montagem do anfitrião"
    Introduza `user/appdata/rest-server/bombvault-containers/containers`, e **não** `/mnt/user/appdata/…`. O BombVault corre num contentor onde o `/mnt` do anfitrião está montado noutro sítio; um caminho absoluto do anfitrião não existe lá. Se colar um, o BombVault indica-lhe agora o caminho relativo a usar.

    O VAULT recebe a palavra-passe restic do TOWER através do grupo ao guardar; ninguém precisa de escrever uma chave.

**5. Torne-o mútuo, se quiser.** Repita os mesmos cinco passos no sentido inverso: um rest-server no TOWER a receber a cópia do VAULT. Cada máquina impõe então a imutabilidade à outra, e nenhuma pode apagar as cópias da outra.

## Recuperação guiada

Um separador **Recuperação** dedicado acompanha uma instalação de raiz ou reconstruída pelo caso de desastre, num só lugar:

1. **Restaura primeiro as próprias definições do BombVault**, para que os caminhos de backup, os destinos externos e as credenciais de que o resto do fluxo precisa venham pré-preenchidos (aplicado através de um reinício automático sobre o socket Docker, para que a base de dados de definições em execução nunca seja sobrescrita sob um handle aberto).
2. **Verifica que o BombVault consegue ler os seus backups** (o senão da chave de encriptação logo à partida).
3. Deixa-o **apontar para o seu repo existente** (local ou externo).
4. **Descobre** os containers, VMs, conjuntos de ficheiros e conjuntos de dados ZFS nele armazenados.
5. **Restaura os containers e as VMs de uma só vez** (deixados parados, para que os inicie deliberadamente) e lista os conjuntos de ficheiros e os elementos ZFS para restaurar um a um; os elementos ZFS voltam desativados. O seu kit de recuperação está a um clique de distância.

!!! tip "Migração planeada versus desastre"
    A recuperação guiada restaura as próprias definições do BombVault a partir de um backup. Para uma mudança *planeada* para uma máquina nova, pode em vez disso levar a sua configuração consigo diretamente com o cartão **Exportar / importar configurações** (um ficheiro JSON portátil). Consulte [Configuração](configuration.md#portable-settings-export-and-import).

### Restaurar a partir de outro repo BombVault {#restore-from-another-bombvault-repo}

Um cartão separado no separador **Recuperação** abre o repo de uma instância BombVault *diferente* (uma partilha montada sob `/mnt`, ou um URL remoto) com a **`APP_KEY` dessa instância**, numa sessão pontual e só de leitura. Navegue pelos containers, VMs e conjuntos de ficheiros lá armazenados, escolha um instantâneo e restaure-o, e o objeto restaurado torna-se um container, VM ou conjunto de ficheiros local normal. Nada é alguma vez escrito no outro repo, e as suas próprias definições de backup ficam intactas (a sessão vive em memória e expira por si própria). Mover um container do servidor A para o servidor B deixa de significar reapontar as suas definições de repo e revertê-las depois. Este cartão é de uso único: abre uma sessão, restaura o que escolher e esquece a outra instância. Se quiser antes um arranjo permanente, em que esta máquina vai buscar segundo um agendamento os instantâneos de outra instância para o seu próprio repositório, isso é o separador **Recolha** da página **Instâncias**.

Um contentor cuja rede não existe neste servidor, como uma rede `br0` do Unraid num host Docker comum, mostra uma escolha de rede por baixo da sua linha. O BombVault cria-o na rede que escolheres, juntamente com as outras redes. O IP fixo e o endereço MAC pertenciam à rede antiga e são descartados, por isso é a nova rede que os atribui.

## Kit de recuperação da chave de encriptação

Esta é a peça que torna a recuperação de desastres possível mesmo quando não existe um BombVault em execução.

Um clique transfere a **chave mestra**, a **palavra-passe restic derivada**, e as **localizações e comandos exatos do repo**, para que possa restaurar diretamente com a CLI do restic em qualquer máquina. Um lembrete no Painel insiste até o ter guardado.

!!! danger "Guarde o kit de recuperação fora do servidor"
    O kit contém o segredo que decifra os seus backups. Guarde-o num local seguro e separado do servidor (um gestor de palavras-passe, uma cópia impressa num cofre). Se perder ambos o BombVault e a `APP_KEY` sem kit de recuperação, os seus backups encriptados não podem ser recuperados.

!!! warning "O snapshot mais recente nem sempre é o que deve restaurar"
    Desde o restic 0.17, `restic snapshots` mostra o tamanho de cada snapshot. Após uma perda de dados, o snapshot mais recente pode ser o que foi esvaziado, por isso não restaure um snapshot muito mais pequeno do que os anteriores. Após um ransomware pode ser o cifrado, com o tamanho habitual. Se o BombVault ainda estiver a correr, veja primeiro a sua página **Anomalias**: ela indica o último backup bom. Um restauro não precisa de nenhum dado de anomalias do BombVault, e a pausa da retenção só mantém mais snapshots.

### Selar o kit

Se ligou a encriptação age para as exportações simples (Definições), o kit também é selado com ela e é transferido como `bombvault-recovery-kit.md.age`. Está em ASCII armor e não em binário, por isso continua a ser texto: colá-lo num gestor de palavras-passe ou imprimi-lo funciona exatamente como antes, só que o conteúdo fica ilegível sem a sua chave.

!!! warning "Não guarde a chave age dentro do kit"
    Precisa da sua chave age **privada** para abrir um kit selado. Guarde-a num sítio que não dependa do próprio kit, ou terá duas coisas para recuperar em vez de uma. Selar compensa quando o kit está guardado num sítio que não controla totalmente (um gestor de palavras-passe partilhado, notas na nuvem, uma cópia impressa num escritório); um kit no seu próprio cofre já está protegido pelo cofre.

    Com a encriptação ligada e nenhum destinatário utilizável configurado, a transferência é simplesmente recusada. O BombVault nunca recorre a entregar a chave mestra em claro.

### Se não tiver o kit à mão

A palavra-passe não está guardada em lado nenhum, é **calculada** a partir da `APP_KEY`. Com a chave e uma shell pode reproduzi-la por si próprio:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

É um HMAC-SHA256 sobre a cadeia fixa `bombvault:restic-repo`, com os bytes crus da `APP_KEY` hexadecimal como chave, impresso como 64 caracteres hexadecimais minúsculos. O mesmo valor está no kit, como palavra-passe restic derivada; isto serve para o dia em que o kit esteja noutro sítio que não junto de si.

!!! warning "Para um repositório recebido, use a chave da instância REMETENTE"
    Um repositório que chegou aqui por replicação fora do local foi criado pela máquina que o enviou, com a **sua** `APP_KEY`. Derivar a partir da chave da máquina recetora dá uma palavra-passe que o restic recusa, o que se lê exatamente como um repositório corrompido sem o ser. É a razão habitual para o `restic check` num repositório recebido pedir a palavra-passe vezes sem conta.

Como as definições de recuperação vivem **dentro** de cada repo (`<repo>/def`, `<repo>/vm-def`), uma pasta de repo copiada é totalmente autossuficiente, por isso o kit mais o repo é tudo o que um restauro em bare-metal precisa.

## Recuperar um dump de base de dados {#database-dumps}

Um dump de base de dados é um ponto de restauro próprio no repositório dos containers, com a etiqueta `dbdump:<container>` e um único ficheiro, `/dbdump/<container>.sql`. O BombVault lista-os, descarrega-os e importa-os em **Backups**; abaixo estão os mesmos passos só com o restic, para o dia em que o BombVault não estiver lá.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

As etiquetas `dbversion:` e `dbname:` de cada dump dizem de que versão de servidor veio e que bases contém. Um ficheiro completo termina com `-- PostgreSQL database cluster dump complete` ou `-- Dump completed`.

Importe-o para um container da mesma versão ou de uma mais recente (PostgreSQL), ou da mesma versão principal (MySQL e MariaDB), arrancado uma vez com a pasta de dados vazia para que se inicialize. O anfitrião não precisa de cliente de base de dados, o container tem um:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Para uma só base dentro de um dump completo, o MySQL e o MariaDB aceitam `--one-database <name>` no comando do cliente. Um dump de PostgreSQL tem uma secção por base, cada uma a começar numa linha `\connect <name>`: copie essa secção para um ficheiro próprio e importe-o com `-d <name>` depois de criar a base.

!!! warning "Um dump feito como root traz as contas do servidor"
    Um dump completo de MySQL ou MariaDB feito como root contém a base de sistema `mysql`, pelo que importá-lo substitui as contas do servidor novo, palavra-passe de root incluída, pelas do dump. No PostgreSQL, `role ... already exists` para o utilizador criado pelo container é esperado e inofensivo.
