# Conjuntos de dados ZFS

Os conjuntos de dados ZFS são entradas na lista **Backups**, como os containers e as VMs. Uma entrada é um conjunto de dados juntamente com todos os conjuntos abaixo dele. Em cada backup o BombVault tira um instantâneo ZFS da árvore inteira, por isso todos os conjuntos de dados nela são capturados no mesmo instante. Depois lê os ficheiros de cada conjunto a partir desse instantâneo e guarda-os com o restic, da mesma forma que uma pasta, e remove o instantâneo logo a seguir. Os backups são deduplicados, podes navegar por todos eles e é possível restaurar ficheiros avulsos.

O backup nunca envia um fluxo para o restic e nunca reverte um conjunto de dados. O BombVault só destrói instantâneos que ele próprio criou. A [réplica](#replica) opcional é o único sítio que usa `zfs send`: copia os conjuntos de dados para um segundo servidor ZFS e não toca no backup.

## Requisitos {#requirements}

- **A ligação SSH a este servidor.** Os conjuntos de dados ZFS usam a mesma chave, o mesmo anfitrião e o mesmo utilizador que os backups de VM. Se os backups de VM já funcionam, isto também funciona. Caso contrário, segue o [guia de backup de VM por SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) no GitHub. Os campos do modelo chamam-se **Host SSH: Address**, **Host SSH: Port** e **Host SSH: User**.
- **O comando `zfs` nesse anfitrião.** O Unraid 6.12 e posteriores e o TrueNAS SCALE têm-no.
- **Host Data mapeado como `/mnt` com o Access Mode Read/Write - Slave.** É o valor predefinido do modelo. O instantâneo de um conjunto de dados só aparece dentro da pasta `.zfs/snapshot` do conjunto de dados depois de o BombVault ter arrancado, por isso o contentor tem de receber as montagens que o anfitrião faz mais tarde.
- **Os conjuntos de dados montados abaixo de `/mnt`.** No Unraid, os pools ficam em `/mnt/<pool>`, por isso isso já acontece.

Ativa a secção em **Definições, Geral** (Conjuntos de dados ZFS). **Ligação ZFS a este servidor** em **Mais** na página Backups abre então **Ligação a este servidor**. Testa a ligação SSH, indica o utilizador e o host a que se liga, e diz o que falta quando algo falta. Quando a ligação está em baixo, a lista Backups di-lo com **Ver ligação**, e o cartão **Integração com o host** na visão geral mostra o mesmo resultado.

## Entradas e conjuntos de dados filhos {#items-and-children}

Escolhe **Adicionar** na página Backups e depois **Um conjunto de dados ZFS**. A lista vem do servidor. Escolhe o conjunto no topo do que queres copiar, por exemplo `cache/appdata`, e a entrada abrange-o e a todos os conjuntos abaixo dele.

- **Os novos conjuntos de dados filhos entram sozinhos.** Um conjunto de dados criado mais tarde abaixo da entrada é guardado na execução seguinte, que o indica como novo. O primeiro backup lê-o por inteiro uma vez; depois só são lidas as alterações.
- **Podes deixar de fora filhos individuais.** Desativa um filho no separador Definições da entrada e ele fica de fora juntamente com tudo o que está abaixo. Um filho deixado de fora que já não existe no servidor é marcado como tal e pode ser retirado da lista.
- **Os filhos que não podem ser lidos são ignorados, nunca em silêncio.** A execução lista-os, a entrada mostra quantos foram ignorados, e o cartão **Sem backup** na visão geral conta cada um como não protegido. A execução continua a copiar tudo o resto e não falha por causa de um filho ignorado. As razões estão na [tabela de códigos de motivo](#reason-codes): um conjunto de dados não montado, com `canmount=off`, com ponto de montagem `legacy` ou sem ele, com uma chave de encriptação não carregada, com o acesso aos instantâneos desligado, ou com um ponto de montagem que o BombVault não vê.
- **Um conjunto de dados ignorado não arrasta os filhos consigo.** Um conjunto de dados com `canmount=off` que só contém outros conjuntos de dados é ignorado (mostrado como "só estrutura") e os seus filhos montados são guardados. Um conjunto de dados encriptado cuja chave não está carregada é ignorado juntamente com os filhos que partilham a sua chave.
- **Os filhos que são discos de VM ou dados do sistema começam desativados** na caixa de diálogo de adição, com o motivo junto ao interruptor. Adicionar um pool inteiro pede uma confirmação que enumera o que ele contém.

### Volumes {#volumes}

Um volume (zvol) contém um disco virtual em vez de ficheiros, e uma entrada ZFS nunca faz backup de um.

- Faz-se backup de um volume que uma VM usa com essa VM.
- Um volume que nenhuma VM usa (um extent iSCSI, um disco que desligaste) **não tem backup do BombVault**. A janela de adicionar e a página Backups contam estes volumes e di-lo. Uma versão futura fará backup deles.

Os volumes dentro da árvore de uma entrada são ignorados e indicados em cada execução.

### O armazenamento do Docker {#docker-storage}

Com o controlador de armazenamento ZFS do Docker, cada camada de imagem é um conjunto de dados com um ponto de montagem `legacy`. A caixa de diálogo de adição agrupa-os numa linha por pai. Uma árvore com mais de 20 desses conjuntos de dados não pode tornar-se uma entrada: enquanto existir um instantâneo dela, o Docker não consegue remover camadas de imagem. Adiciona antes os conjuntos de dados abaixo dela, por exemplo `appdata`.

### Os entradas nunca se sobrepõem {#overlap}

Um conjunto de dados só pode pertencer a uma entrada. O BombVault recusa uma entrada nova que fique dentro de uma existente ou que contivesse uma. Para juntar várias entradas filhas numa entrada pai, elimina primeiro as entradas filhas escolhendo manter os seus backups e depois adiciona o pai. Cada conjunto de dados mantém o seu histórico com o próprio nome, por isso o backup seguinte continua onde as entradas antigas ficaram e não volta a ler tudo.

## Parar contentores e correr comandos à volta do instantâneo {#consistency}

Um instantâneo de uma base de dados em execução é como uma falha de energia súbita: a base de dados normalmente recupera, mas tem de o fazer. Cada entrada pode fazer duas coisas quanto a isso, e ambas cobrem apenas o instante do instantâneo, não o backup inteiro.

- **Parar estes contentores para o instantâneo.** O BombVault para os contentores da lista, tira o instantâneo e volta a arrancá-los de imediato. Os contentores de um mesmo nível de dependências param em paralelo, primeiro os dependentes, por isso a janela inteira dura normalmente alguns segundos; a execução mostra quanto durou. O backup lê depois o instantâneo congelado enquanto as aplicações já estão outra vez a correr. Só são parados os contentores que estavam a correr.
- **Um comando antes e depois do instantâneo.** Corre dentro de um contentor à tua escolha, por exemplo para exportar uma base de dados para o conjunto de dados mesmo antes do instantâneo, sem parar nada. Se o comando antes do instantâneo falhar, o backup falha e não é tirado nenhum instantâneo. Um comando depois do instantâneo que falhe é mostrado na execução mas não faz falhar o backup.

O que acontece quando algo corre mal:

- Se um contentor não puder ser parado, o BombVault arranca os que já parou e o backup falha, indicando o contentor. Nunca recorre a um instantâneo de aplicações em execução.
- A paragem espera que termine um backup de contentor em curso (até 30 minutos numa execução manual, até ao limite de tempo do backup numa agendada), para que os dois nunca parem e arranquem o mesmo contentor ao mesmo tempo.
- Antes de o primeiro contentor parar, o BombVault anota quais para. Se o BombVault for terminado dentro da janela, volta a arrancar esses contentores no seu arranque seguinte, envia uma notificação e a entrada mostra uma nota vermelha para cada contentor que não conseguiu arrancar.

Os dumps automáticos de bases de dados (ver [Funcionalidades](features.md)) correm com o backup do próprio container, não com uma entrada ZFS. Uma base de dados cujo container só tem backup através do seu conjunto de dados não recebe dump, por isso dá-lhe aqui um comando.

Um container pode estar nesta lista e ter backup como container ao mesmo tempo. Os seus dados são então guardados duas vezes, em dois repositórios, e o backup total pára-o duas vezes. A entrada di-lo.

## Restaurar {#restore}

Abre a entrada e escolhe **Restauração**, ou escolhe-a na página **Restauração**. A janela de restauro pede o backup e depois o **Conjunto de dados**. Por predefinição é o conjunto de topo da entrada. Em **O quê** restauras tudo ou só os ficheiros e pastas que escolheres. **Para onde** oferece:

- **Restaurar dentro do conjunto de dados.** Os ficheiros do backup são escritos no ponto de montagem do conjunto de dados. Os ficheiros com o mesmo nome são substituídos, os outros ficam. O conjunto de dados nunca é revertido nem substituído. O BombVault verifica que o conjunto de dados está montado, visível e gravável, uma vez antes de começar e de novo mesmo antes de escrever. Onde um conjunto de dados filho estiver montado lá dentro, nada é escrito: o filho mantém os seus ficheiros, o dono e as permissões, e é restaurado a partir do seu próprio backup.
- **Para uma pasta.** Escolhe uma pasta abaixo de `/mnt`. O BombVault verifica que a pasta está num pool ou partilha montados e que há espaço livre suficiente. Isto funciona sem a ligação SSH e para conjuntos de dados que já não existem.
- **Num novo conjunto de dados.** Indica um conjunto de dados que ainda não existe; a janela sugere `<dataset>-restored`. O BombVault cria-o com as propriedades ZFS guardadas no backup e restaura para ele, ver [Restaurar como novo conjunto de dados](#new-dataset).

Escolher **Todos os conjuntos de dados deste backup** como conjunto de dados escreve cada conjunto da árvore na sua própria subpasta da pasta que escolheres. Os conjuntos ignorados nesse backup são nomeados. Para restaurar a partir do repositório de outro BombVault, usa **De uma pasta de backups antiga** na página Restauração; restaura sempre para uma pasta: todos os conjuntos de um backup, cada um na sua subpasta, ou um conjunto da árvore, inteiro ou com ficheiros escolhidos.

A lista de containers a parar da entrada também é oferecida num restauro para dentro do conjunto de dados. Esses containers ficam parados durante todo o restauro, e os backups de containers esperam entretanto.

### O instantâneo de segurança {#safety-snapshot}

Antes de escrever num conjunto de dados, o BombVault tira um instantâneo ZFS só desse conjunto, chamado `bombvault-prerestore-<time>`. **Guardar primeiro um instantâneo ZFS do estado atual** vem ligado por predefinição; desligá-lo exige uma segunda confirmação. Se o instantâneo não puder ser tirado, nada é restaurado.

O BombVault nunca apaga sozinho um instantâneo de segurança. A entrada enumera-os com a idade e o tamanho, cada um com a ação **Excluir**, e avisa quando o mais antigo tem mais de 30 dias, porque retém no pool dados apagados e alterados.

Para voltar atrás depois de um restauro, copia ficheiros individuais de `.zfs/snapshot/bombvault-prerestore-<hora>` dentro do conjunto de dados. `zfs rollback <dataset>@bombvault-prerestore-<hora>` só funciona enquanto for o instantâneo mais recente desse conjunto de dados. `zfs rollback -r` apaga todos os instantâneos mais recentes, incluindo os automáticos.

### Restaurar como conjunto de dados novo {#new-dataset}

O BombVault guarda em cada backup as propriedades ZFS definidas localmente de cada conjunto de dados: compression, recordsize, quota, reservation, atime, xattr, acltype, casesensitivity e as suas próprias propriedades de utilizador. Os valores herdados e só de leitura ficam de fora, porque voltam sozinhos. Os backups de antes de o BombVault as guardar não têm nenhuma.

- **Num conjunto de dados novo** executa `zfs create` com cada propriedade guardada. casesensitivity, normalization e utf8only só podem ser definidas assim. As quotas e reservas são definidas depois dos ficheiros, para não os poderem recusar. O ponto de montagem fica de fora para a cópia não colidir com o original, tal como `canmount`, `readonly` e a encriptação, para o restauro poder escrever. Um conjunto de dados novo abaixo de um encriptado herda a sua encriptação. O conjunto de dados acima tem de existir. Se algo falhar depois de criado, o conjunto de dados novo fica no servidor, porque o BombVault nunca destrói um conjunto de dados.
- **Restaurar dentro do conjunto de dados** mostra as propriedades guardadas junto ao restauro. **Definir também estas propriedades** define as que um conjunto de dados existente ainda aceita, antes de escrever qualquer ficheiro. As quotas e reservas são definidas depois dos ficheiros, para não os poderem recusar. Sem esse interruptor, o conjunto de dados mantém as suas definições.

## O que está no backup {#contents}

No backup: os ficheiros e pastas de cada conjunto de dados guardado, com o dono, as permissões, as datas e os atributos estendidos tal como o restic os guarda, e as propriedades ZFS definidas localmente de cada conjunto de dados.

Não está no backup:

- o dono e as permissões da própria pasta de topo de cada conjunto de dados (tudo o que está abaixo dela está incluído). Um restauro dentro do conjunto de dados deixa a pasta de topo existente como está; um restauro para uma pasta cria-a com as permissões `0755`;
- os instantâneos ZFS existentes;
- os filhos que foram ignorados ou deixados de fora;
- os volumes.

Para restaurar num pool novo, cria o pool e restaura cada conjunto de dados num conjunto de dados novo. Ainda não foi verificado se as ACL NFSv4, tal como o TrueNAS as usa em conjuntos de dados SMB, voltam como esperas, por isso testa um restauro com os teus próprios dados antes de confiares nelas.

## Conjuntos de dados encriptados {#encryption}

Um conjunto de dados encriptado só é guardado enquanto a sua chave estiver carregada. Caso contrário, é ignorado com um aviso; carrega a chave com `zfs load-key` e monta o conjunto de dados. O BombVault lê os dados desencriptados e guarda-os no repositório do restic, que é encriptado. Se desligaste a encriptação no BombVault, esse repositório não o é.

## Réplica {#replica}

Uma réplica é uma cópia dos conjuntos de dados de uma entrada num segundo servidor ZFS. O BombVault mantém-na atualizada com `zfs send` e `zfs receive`. A primeira execução envia tudo; depois só viajam os blocos alterados. No outro servidor podes montar a cópia de imediato.

Uma réplica nunca substitui o backup. As versões antigas, os ficheiros individuais e a verificação continuam a vir dos backups, e a réplica guarda apenas tantos instantâneos quantos definir. Uma réplica atual conta como cópia fora do local, mas uma entrada com réplica e sem backup continua laranja.

**Só réplica, sem backup** no cartão Réplica faz com que toda a secção ZFS envie réplicas e não faça backups. A pasta própria e os destinos externos mantêm os seus interruptores, por isso ao desligá-lo a secção volta a fazer backup para eles, e o cartão de cada entrada ZFS mostra o interruptor da secção tal como está. Uma entrada assim espera pela primeira réplica em vez de um backup, **Fazer backup agora** passa a **Replicar agora**, e à volta do instantâneo os seus containers param e os seus comandos correm como num backup. As suas réplicas contam para um agendamento de poucos em poucos dias, para o alerta de atraso e para a cobertura.

Ativa-a no cartão **Réplica** do separador Definições da entrada. Aí escolhes para onde vai a réplica, quando corre (**Depois de cada backup** ou **Plano próprio**) e quantos instantâneos ficam no alvo e neste servidor. O cartão lista cada conjunto de dados e volume com o seu estado, e **Replicar agora** inicia uma execução. Uma execução de réplica tem um bloqueio próprio, por isso uma primeira transferência longa nunca atrasa os backups.

**Replicar tudo agora**, ao lado das ações de backup quando o mosaico ZFS está escolhido na página Backups, envia a réplica de cada entrada ZFS, uma a seguir à outra. O servidor executa o lote, por isso continua quando sais da página, e uma entrada que não consegue replicar é indicada e saltada.

### Enviar para um servidor ZFS {#replica-push}

Qualquer máquina com ZFS e SSH pode receber, por exemplo um segundo Unraid ou um TrueNAS. O BombVault não tem de correr lá.

1. Em **Definições, Emparelhamento**, escolhe **Ligar por SSH** em **Servidor ZFS sem BombVault**, ou **Adicionar servidor ZFS** no cartão **Réplica** da entrada.
2. Indica o endereço, o utilizador e a porta. A caixa de diálogo mostra a chave pública do BombVault. Adiciona-a ao `~/.ssh/authorized_keys` do utilizador no servidor. No Unraid fica em **Users, root, SSH keys**. Conforme **Display Settings**, Users fica no menu do cabeçalho ou em Settings. No TrueNAS, edita o utilizador em **Credentials, Users**. Esta é a chave da réplica. É diferente da chave da ligação SSH das VMs e do anfitrião, por isso adicione-a mesmo que essa já esteja autorizada.
3. Testa a ligação. A caixa de diálogo lista então os pools do servidor. Escolhe um e define a raiz, que por predefinição é `<pool>/bombvault-replica`.
4. Escolhe o novo servidor no cartão **Réplica** da entrada. O servidor tem então um cartão próprio na página Instâncias, e **Abrir** leva à sua página.

Com root não é preciso mais nada. Um utilizador próprio precisa destas permissões no pool do destino, que a caixa de diálogo também mostra:

```
zfs allow <user> receive,create,mount,rollback,destroy,userprop <pool>
```

Na origem, um utilizador do mesmo tipo precisa destas no conjunto de dados de topo da entrada:

```
zfs allow <user> send,snapshot,hold,release,bookmark,destroy <dataset>
```

Neste sentido, o BombVault que detém a entrada também detém uma chave que pode escrever no servidor.

O BombVault guarda a chave de anfitrião que o servidor mostra na primeira ligação e depois recusa qualquer outra. Se o servidor foi reinstalado, carrega em **Esquecer chave de anfitrião** na página do servidor, e a ligação seguinte guarda a nova chave.

Quando o anfitrião deste servidor consegue chegar ele próprio ao servidor ZFS, encaminha o `zfs send` diretamente para o seu próprio ssh até ao servidor, com a chave da réplica e a chave de anfitrião guardada, por isso o fluxo não passa pelo BombVault. Caso contrário, o fluxo passa pelo BombVault.

### Enviar para uma instância emparelhada {#replica-receive}

Um BombVault emparelhado pode receber a réplica ele próprio. Ninguém recebe acesso SSH ao outro anfitrião, e nenhuma chave vai para um ficheiro `authorized_keys`.

1. No cartão **Réplica** da entrada, escolhe a instância emparelhada como alvo, ou liga a função **Servidor ZFS** no cartão dessa instância na página Instâncias e escolhe aí as entradas ZFS. O cartão mostra **A aguardar aprovação** até ela responder.
2. Na instância recetora, abre **Instâncias**. O pedido aparece no cartão da instância emissora como distintivo, e os seus **Detalhes** listam-no. Escolhe o pool e a raiz nesse servidor e quantos instantâneos ficam, depois prime **Permitir** ou **Recusar**.
3. Depois de **Permitir**, a origem envia segundo o seu próprio plano, como para qualquer outro destino.

A instância recetora aceita apenas o que a aprovação cobre: os conjuntos de dados da entrada, na sua própria raiz. É ela que executa `zfs receive`, e a origem não tem forma de apagar nem de reverter nada aí. Por isso a cópia sobrevive a uma origem que alguém tenha tomado. A instância recetora mantém a sua própria retenção. A origem só sugere uma regra quando faz o pedido.

**Revoke access** na instância recetora termina a aprovação a qualquer momento e avisa a origem, que então mostra que a aprovação foi retirada e para. O que a instância recetora já tem fica lá. Um pedido recusado continua recusado. Mais conjuntos de dados, ou um pedido depois de uma revogação, voltam a perguntar.

A instância recetora tem de responder por HTTPS. Para um endereço http simples a origem não envia nada, porque o token e os dados atravessariam a rede sem cifra. Cada transferência deixa ainda livre um décimo do pool recetor e para antes de lhe tocar, por isso uma única origem não consegue encher o pool do servidor recetor.

### Onde ficam os dados {#replica-target}

Cada conjunto de dados fica em `<root>/<server>/<pool>/<path>`. A pasta do servidor é o nome da instância de origem, fixado na primeira transferência, por isso dois servidores com o mesmo nome de pool nunca se atropelam. Por exemplo, `cache/appdata` de um servidor chamado `tower` acaba em `backup/bombvault-replica/tower/cache/appdata`.

Depois de o BombVault ser reinstalado, o alvo recusa a pasta que este BombVault preenchia antes. O cartão Réplica oferece então **Assumir a pasta**, e a réplica seguinte continua a partir das cópias que lá estão. Assume-a só se nenhum outro BombVault ainda replicar para essa pasta.

A cópia no destino é só de leitura e não está montada, por isso nunca cobre nada nesse servidor. As propriedades ZFS seguem juntas, exceto o ponto de montagem, `sharenfs` e `sharesmb`. As reservas `reservation` e `refreservation` também ficam de fora, por isso a cópia só ocupa o espaço de que os seus dados precisam.

### O que entra {#replica-contents}

Entra tudo o que a entrada guarda no backup, e também os volumes abaixo dele, que o backup ignora. Um conjunto de dados filho que desativou na entrada fica de fora. Todos os conjuntos de dados de uma execução vêm de um único instantâneo, como no backup.

### Quanto tempo ficam os instantâneos {#replica-retention}

No destino, uma réplica nova guarda 7 instantâneos diários e 3 semanais. Escolhe antes **Short**, **Balanced** ou **Long**, ou define **Custom values**. Só são removidos aí os instantâneos com o nome `bombvault-replica-<14 digits>`, e nunca o mais recente que os dois lados partilham.

Na origem, o BombVault guarda o instantâneo de réplica mais recente, mais um marcador para cada estado que enviou. Os marcadores não ocupam espaço. A transferência seguinte parte deles.

Para guardares mais também na origem, define **Manter neste servidor** no cartão Réplica, por exemplo para 7. Depois de cada réplica ficam então neste servidor tantos dos instantâneos de réplica mais recentes quantos indicares, além do que o alvo guarda. 0 ou 1 guarda só o mais recente. O mais recente tem um hold, para que a base da transferência seguinte não seja apagada por engano. Os mais antigos não têm nenhum. Cada instantâneo prende os blocos que mudaram depois dele, por isso cada um que guardas ocupa espaço no pool. Desligar a réplica ou mudá-la para outro alvo, ou apagar a entrada, também os remove deste servidor.

Podes ler ou copiar de imediato os ficheiros de um instantâneo guardado, a partir da pasta `.zfs/snapshot` dentro do conjunto de dados ou de um clone, sem tocar no conjunto de dados nem na réplica:

```
ls /mnt/cache/appdata/.zfs/snapshot/bombvault-replica-20261006014100
zfs clone cache/appdata@bombvault-replica-20261006014100 cache/appdata-20261006
```

Um clone não ocupa espaço até algo nele mudar. Apaga-o com `zfs destroy cache/appdata-20261006` quando terminares, porque um instantâneo que tem um clone não pode ser removido.

Para voltar ao instantâneo de réplica mais recente basta aqui `zfs rollback cache/appdata@<último>`, e no alvo não há nada a fazer. Reverter o conjunto de dados inteiro para um mais antigo apaga todos os instantâneos posteriores, neste servidor e no alvo. O mais recente tem um hold, por isso tira-o primeiro e depois reverte aqui o conjunto de dados:

```
zfs release -r bombvault-replica cache/appdata@bombvault-replica-20261008014200
zfs rollback -r cache/appdata@bombvault-replica-20261006014100
```

A cópia no alvo também tem de voltar atrás, mas nem sempre para o mesmo instantâneo, porque a retenção própria do alvo pode já o ter removido. Executa a réplica uma vez. Ela para nesse conjunto de dados porque o alvo tem instantâneos mais recentes, e o cartão Réplica e o histórico de execuções indicam o instantâneo para o qual deves reverter o alvo: o mais recente que os dois lados ainda partilham. Reverte a cópia para esse instantâneo no servidor ZFS, ou no anfitrião da instância emparelhada que recebe a réplica, por exemplo:

```
zfs rollback -r backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261005014000
```

`zfs rollback` atua num só conjunto de dados, por isso repete as reversões para cada conjunto de dados filho que também queiras recuperar. Depois disso, **Replicar agora** continua a partir do instantâneo partilhado.

### Conjuntos de dados encriptados numa réplica {#replica-encryption}

Um conjunto de dados encriptado é enviado em bruto. Continua encriptado no destino, e o destino nunca vê a chave. Guarda bem a chave: precisas dela para abrir a cópia depois de um restauro, e uma réplica sem ela é ilegível.

### Usar a réplica {#replica-use}

Abre o separador **Backups** da entrada e clica na linha **Réplica ZFS** entre os locais que guardam os seus instantâneos. A folha lista os instantâneos no alvo e mostra os comandos com os teus nomes reais.

Para ver um estado antigo, clona um instantâneo no destino. Um clone não ocupa espaço até algo mudar, e a réplica fica intacta:

```
zfs clone backup/bombvault-replica/tower/cache/appdata@bombvault-replica-20261006014100 backup/bombvault-replica/clone-appdata
```

Se a origem falhar, transforma a cópia num conjunto de dados normal e gravável no destino:

```
zfs inherit -r readonly backup/bombvault-replica/tower/cache/appdata && zfs inherit -r canmount backup/bombvault-replica/tower/cache/appdata && zfs mount -a
```

Depois disso, o BombVault deixa de replicar para esse conjunto de dados, até iniciar uma nova primeira execução.

Para recuperar um estado na origem, carrega em **Bring back as a new dataset** na folha. O BombVault envia o instantâneo para um conjunto de dados novo ao lado do original, com o nome `<dataset>-bombvault-restore-` mais uma marca temporal. Nunca escreve por cima do original.

## Instantâneos que sobraram {#leftover-snapshots}

O instantâneo de um backup chama-se `<dataset>@bombvault-<14 dígitos>`, por exemplo `cache/appdata@bombvault-20260924021500` (UTC). O BombVault remove-o logo após o backup. Se isso falhar, por exemplo porque o conjunto de dados está ocupado ou o BombVault foi parado, o BombVault remove-o:

- antes do backup seguinte desse entrada,
- quando o BombVault arranca, para cada entrada, mesmo com a secção desligada,
- quando elimina a entrada,
- quando carrega em **Remover agora** na entrada, que mostra quantos restam.

Só são removidos os nomes que correspondem exatamente a `bombvault-` mais 14 dígitos. Os instantâneos de segurança, os teus próprios instantâneos e os automáticos nunca são tocados. Para remover um à mão:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

O instantâneo de uma réplica chama-se `<dataset>@bombvault-replica-<14 digits>` e não é um resto. Fica na origem até uma execução de réplica mais recente o substituir, ou enquanto **Manter neste servidor** o mantiver, e no destino enquanto a retenção o mantiver. O limpador nunca lhe toca, porque só corresponde a `bombvault-` seguido de exatamente 14 dígitos.

## Anomalias {#anomalies}

Um filho que foi esvaziado mal altera o total de uma árvore grande, por isso a deteção de anomalias vigia cada conjunto de dados de uma entrada por si: o tamanho, o número de ficheiros, os dados novos e o tempo do restic têm cada um o seu próprio histórico. Esse histórico pertence ao nome do conjunto de dados, por isso mantém-se quando a árvore é mais tarde guardada por outra entrada.

Um conjunto de dados que a execução anterior guardou e que esta não conseguiu ler conta como esvaziado, desde que a seleção da entrada não tenha mudado. Isso abrange uma chave não carregada, um conjunto de dados não montado e um que desapareceu da árvore. Um filho que exclui por si próprio altera a seleção, por isso o seu histórico recomeça do zero. Enquanto uma deteção de dados perdidos estiver aberta, a retenção mantém os backups antigos apenas desse conjunto de dados e limpa o resto da árvore como de costume.

Cada conjunto de dados tem uma linha própria na monitorização da entrada, que se abre com **Monitorização** na página **Anomalias** e está no separador **Verificação** da entrada. As deteções abertas aparecem junto de cada conjunto. A ligação numa deteção abre a janela de restauro no último backup bom do conjunto. Se uma execução termina é avaliado para a entrada inteira, porque uma execução tem êxito ou falha como um todo.

As verificações em si estão descritas em [Funcionalidades](features.md). Um assistente ligado através do [servidor MCP](mcp.md) pode listar os pontos de restauro de uma entrada ZFS, iniciar o seu backup e ler as deteções, mas confirmar uma deteção faz-se na página **Anomalias**.

## Códigos de motivo {#reason-codes}

A página da entrada, o histórico de execuções e as notificações nomeiam um problema com um destes códigos. A maioria tem também a correção ao lado na página.

| Código | Significado | O que fazer |
|---|---|---|
| `ssh-missing` | A ligação SSH não está configurada neste contentor. | Configura a ligação SSH como para os backups de VM. |
| `host-placeholder` | Host SSH: Address continua a ser o valor de exemplo, e `host.docker.internal` também não respondeu. | Define Host SSH: Address como o IP LAN deste servidor. |
| `host-fallback` | Host SSH: Address continua a ser o valor de exemplo, e `host.docker.internal` funciona. | Nada, ou define o IP LAN. |
| `ssh-unreachable` | Não é possível chegar ao servidor por SSH. | Verifica o endereço e a porta, e que o SSH está ligado. |
| `ssh-auth` | O servidor recusou a chave do BombVault. | Executa uma vez no servidor o comando mostrado em **Ligação a este servidor**. |
| `zfs-not-found` | O anfitrião SSH não tem o comando `zfs`. | Aponta Host SSH: Address para a máquina a que pertencem os pools. |
| `zfs-permission` | O utilizador SSH não pode executar este comando zfs. | Usa root, ou vê [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` indica outro anfitrião ou utilizador que os campos SSH. | Põe-nos de acordo, ou esvazia os campos SSH para que ambos venham do URI. |
| `zfs-error` | O zfs reportou outro erro. | Os detalhes mostram a sua mensagem. |
| `propagation-missing` | As novas montagens no anfitrião não chegam ao contentor. | Define o Access Mode de Host Data como Read/Write - Slave e reinicia o BombVault. |
| `invalid-name` | Um nome de conjunto de dados que o BombVault não aceita. | Muda o nome do conjunto de dados. |
| `name-too-long` | Um conjunto de dados da árvore é demasiado longo para um nome de instantâneo. | Muda-lhe o nome, ou adiciona como entrada um conjunto de dados abaixo dele. |
| `invalid-exclude` | Um padrão de exclusão ou um filho excluído não encaixa na entrada. | Corrige a entrada indicada na mensagem. Para deixar de fora um conjunto de dados filho inteiro, desativa-o em vez de escrever um padrão. |
| `not-found` | O conjunto de dados não existe no servidor. | Remove a entrada ou volta a criar o conjunto de dados. Os seus backups continuam restauráveis. |
| `not-filesystem` | Isto é um volume, não um sistema de ficheiros. | Vê [Volumes](#volumes). |
| `overlaps-item` | O conjunto de dados sobrepõe-se a uma entrada existente. | Vê [Os entradas nunca se sobrepõem](#overlap). |
| `docker-storage` | A árvore contém o armazenamento de imagens do Docker. | Vê [O armazenamento do Docker](#docker-storage). |
| `nothing-readable` | Neste momento nenhum conjunto de dados da entrada pode ser lido. | Vê os códigos dos conjuntos de dados ignorados. |
| `snapshot-failed` | Não foi possível criar o instantâneo. | Os detalhes mostram a mensagem do zfs. |
| `containers-busy` | Um backup de contentor ainda estava a correr quando os contentores tinham de parar. | Volta a tentar mais tarde. As execuções agendadas esperam sozinhas. |
| `consistency-stop-failed` | Não foi possível parar um contentor, por isso não foi tirado nenhum instantâneo. | Verifica o contentor, ou retira-o da lista. |
| `pre-snapshot-failed` | O comando antes do instantâneo falhou. | Os detalhes da execução mostram a sua saída. |
| `container-unknown` | Um contentor da lista não existe. | Retira-o da lista. |
| `container-is-self` | O BombVault não pode parar o próprio contentor. | Retira-o da lista. |
| `leftover-snapshots` | Continuam no servidor instantâneos que o BombVault não conseguiu remover. | Carrega em **Remover agora**, vê [Instantâneos que sobraram](#leftover-snapshots). |
| `zvol` | Um volume na árvore, ignorado. | Vê [Volumes](#volumes). |
| `canmount-off` | Nunca montado (`canmount=off`), ignorado. | Se tiver dados, monta-o ou move os dados para um conjunto de dados filho. |
| `legacy-mount` | Ponto de montagem legacy, ignorado. | Dá-lhe um ponto de montagem abaixo de `/mnt`. |
| `no-mountpoint` | Sem ponto de montagem, ignorado. | Dá-lhe um ponto de montagem abaixo de `/mnt`. |
| `not-mounted` | Não montado no servidor, ignorado. | Monta-o com `zfs mount`, ou define `canmount=on`. |
| `key-not-loaded` | Encriptado e a chave não está carregada, ignorado. | `zfs load-key` e depois monta-o. |
| `snapdir-disabled` | O acesso a instantâneos está desligado, ignorado. | `zfs set snapdir=hidden <dataset>`. A pasta `.zfs` continua oculta. |
| `not-visible` | O BombVault não vê o ponto de montagem do conjunto de dados. | Move o ponto de montagem para baixo do caminho de Host Data, ou mapeia-o no contentor no mesmo caminho com Read/Write - Slave. |
| `shfs-only` | O conjunto de dados só é visível através de `/mnt/user`, que esconde os instantâneos. | Mapeia `/mnt`, e não `/mnt/user`, como Host Data. |
| `snapshot-not-visible` | O instantâneo foi criado mas não apareceu dentro do BombVault. | Executa **Testar o acesso aos instantâneos**; vê abaixo. |
| `snapshot-loop` | O instantâneo não chegou ao BombVault porque Host Data não deixa passar montagens novas. | Define o Access Mode de Host Data como Read/Write - Slave e reinicia o BombVault. |
| `backup-failed` | O restic falhou para este conjunto de dados. | Os detalhes da execução mostram porquê. |
| `not-reached` | A execução terminou antes deste conjunto de dados. | Volta a executar o backup. |
| `gone` | O conjunto de dados já não está no servidor. | Nada. Os seus backups continuam restauráveis. |
| `read-only-mount` | O BombVault só consegue ler o conjunto de dados, por isso não pode restaurar para dentro dele. | Define o mapeamento como Read/Write - Slave, ou restaura para uma pasta. |
| `destination-not-mounted` | A pasta não está num pool ou partilha montados. | Escolhe uma pasta num pool ou partilha. |
| `not-enough-space` | Não há espaço livre suficiente no destino. | Liberta espaço ou escolhe outra pasta. |
| `safety-snapshot-failed` | Não foi possível tirar o instantâneo de segurança, por isso nada foi restaurado. | Os detalhes mostram a mensagem do zfs. |
| `safety-name-too-long` | O nome do conjunto de dados é demasiado longo para um instantâneo de segurança. | Desliga o instantâneo de segurança, ou restaura para uma pasta. |
| `dataset-exists` | Já existe um conjunto de dados com este nome. | Escolhe outro nome, ou restaura para dentro do próprio conjunto de dados. |
| `create-failed` | Não foi possível criar o novo conjunto de dados. | Os detalhes mostram a mensagem do zfs. Verifica se o conjunto de dados acima existe. |
| `new-dataset-not-visible` | O novo conjunto de dados foi criado, mas o BombVault não o vê, por isso nada foi restaurado. | O conjunto de dados fica no servidor. Monta-o abaixo do caminho Host Data e restaura para dentro dele. |
| `set-properties-failed` | Não foi possível definir as propriedades guardadas, por isso nada foi restaurado. | Os detalhes mostram a mensagem do zfs. |
| `set-limits-failed` | Os ficheiros foram restaurados, mas não foi possível definir a quota ou a reserva guardada. | Os detalhes mostram a mensagem do zfs. Define a quota ou a reserva manualmente com `zfs set`. |

### Verificar o que o contentor vê {#mountinfo}

**Testar o acesso aos instantâneos** no separador Definições da entrada tira um instantâneo real da sua árvore, procura-o dentro do BombVault para cada conjunto de dados e remove-o outra vez. É a forma mais rápida de provar o caminho todo antes da primeira execução agendada.

Para veres por ti, executa isto no servidor:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Cada linha é uma montagem dentro do contentor. A linha de um conjunto de dados mostra o seu caminho dentro do contentor (abaixo de `/host/user`) e o nome do conjunto de dados. Um campo `master:N` nessa linha significa que a montagem recebe as montagens que o anfitrião faz mais tarde, que é o que o acesso aos instantâneos precisa. Se faltar, define o Access Mode de Host Data como Read/Write - Slave e reinicia o BombVault.

## TrueNAS SCALE {#truenas}

- Quando `LIBVIRT_URI` está definido (como para os backups de VM no TrueNAS), o BombVault vai buscar ao URI o anfitrião, o utilizador e a porta SSH dos seus comandos zfs, cada um que não esteja definido à parte. Sem backups de VM não é preciso definir nada: `host.docker.internal` não resolve numa app do TrueNAS, por isso o BombVault chega ao anfitrião pelo gateway predefinido da app, como root na porta 22. Define `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` e `LIBVIRT_SSH_PORT` quando quiseres outro endereço ou outro utilizador, no `environment` da app.
- Adiciona a chave SSH do anfitrião de **Definições, Integrações, SSH do anfitrião** ao root em **Credentials, Users**, e ativa o serviço SSH em **System, Services**. Uma [réplica](#replica-push) para este ou outro TrueNAS precisa também da chave da réplica, que é outra.
- Um utilizador que não seja root precisa de permissão no conjunto de dados de topo da entrada, que cobre então todos os conjuntos de dados abaixo dele:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Restaurar num conjunto de dados novo precisa também de `create` no conjunto de dados acima, e definir propriedades guardadas precisa de permissão para essas propriedades.

  Uma sessão SSH sem root no TrueNAS não tem `/usr/sbin` no caminho; o BombVault chama então `/usr/sbin/zfs` diretamente.
- O **Host Data** da app tem de ser um caminho do anfitrião acima dos conjuntos de dados, por exemplo `/mnt/tank`, e não um ixVolume. Com um caminho do anfitrião, a app passa ao BombVault as montagens novas do anfitrião (`rslave`), que é o que o acesso aos instantâneos precisa. Consulta [TrueNAS SCALE](getting-started.md#truenas) para a configuração completa da app.
