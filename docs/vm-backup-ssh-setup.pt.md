# Backups de VMs

O BombVault faz backup de VMs KVM/libvirt sem montar nenhum caminho do libvirt. Corre `virsh` no host por SSH (`qemu+ssh://`), lê e escreve os discos das VMs através da montagem Host Data (`/mnt` no host é `/host/user` no container) e copia a NVRAM UEFI por SSH. Como nada que pertence ao libvirt é montado por bind, o BombVault nunca pode atrapalhar o Gestor de VMs do Unraid.

Este guia cobre a configuração, os métodos de backup, as VMs com instantâneos do Unraid e o TrueNAS SCALE. Os backups de containers não precisam de nada disto.

Os backups de conjuntos de dados ZFS usam a mesma ligação SSH, com a mesma chave, o mesmo host e o mesmo utilizador. Quando os backups de VMs se ligam, as entradas de conjuntos de dados ZFS ligam-se também. Vê [Conjuntos de dados ZFS](zfs-datasets.md).

## O que configuras {#what-you-configure}

| Definição | Onde | Predefinição | Significado |
|---|---|---|---|
| **Host SSH: Address** (`LIBVIRT_HOST`) | variável do template | `host.docker.internal` | O host Unraid, alcançado por SSH |
| **Host SSH: Port** (`LIBVIRT_SSH_PORT`) | variável do template | `22` | A porta SSH do Unraid |
| **Host SSH: User** (`LIBVIRT_SSH_USER`) | variável do template | `root` | O utilizador SSH no host |
| Chave pública | **Definições, Integrações, SSH do anfitrião** | criada no primeiro arranque | Autoriza-a no host |

O BombVault cria o seu par de chaves SSH no primeiro arranque em `/config/ssh/id_ed25519`, guardado no appdata. A chave do host fica fixada em `/config/ssh/known_hosts` na primeira ligação.

O template preenche **Host SSH: Address** com o marcador `192.168.x.x`. O BombVault trata isso como não definido e tenta `host.docker.internal`, que funciona na rede `bridge` predefinida. Numa rede personalizada, substitui-o pelo IP LAN do host (passo 3 B).

## Passo 1: ativar o SSH no host Unraid {#step-1}

1. No Unraid, abre **Settings, Management Access** e põe **Use SSH** em **Yes**.
2. Se alteraste a porta SSH, anota-a. **Host SSH: Port** tem de corresponder.

## Passo 2: autorizar a chave pública do BombVault {#step-2}

O `/root` do Unraid vive na memória e é apagado em cada arranque, por isso a chave tem de estar também na unidade flash. Corre isto no terminal do Unraid:

```sh
K=$(docker exec BombVault cat /config/ssh/id_ed25519.pub)
mkdir -p /root/.ssh /boot/config/ssh && chmod 700 /root/.ssh
grep -qxF "$K" /root/.ssh/authorized_keys 2>/dev/null  || echo "$K" >> /root/.ssh/authorized_keys
grep -qxF "$K" /boot/config/ssh/root.pubkeys 2>/dev/null || echo "$K" >> /boot/config/ssh/root.pubkeys
chmod 600 /root/.ssh/authorized_keys
```

O Unraid 6.9 e mais recentes copiam `/boot/config/ssh/root.pubkeys` para o `authorized_keys` do root em cada arranque, por isso isto sobrevive a reinícios. Vale para qualquer início de sessão `root`, seja qual for o nome de host ou a porta que o BombVault usa.

## Passo 3: rede {#step-3}

O BombVault tem de abrir uma ligação TCP do container para a porta SSH do host. Escolhe o caso que corresponde à tua configuração.

### A. BombVault na rede bridge predefinida {#bridge}

- Deixa **Host SSH: Address** em `host.docker.internal` ou no marcador do template. O template acrescenta `--add-host=host.docker.internal:host-gateway`, que aponta para o gateway do docker0, que é o host.
- Põe **Host SSH: Port** na tua porta SSH.

### B. BombVault numa rede personalizada {#custom-network}

Um container em `br0.x`, macvlan ou ipvlan não consegue alcançar o host por `host.docker.internal`, porque 172.17.0.1 é o docker0 e não é alcançável a partir de `br0.x`. Em vez disso:

1. No Unraid, põe **Settings, Docker, Host access to custom networks** em **Enabled**.
2. Põe **Host SSH: Address** no IP LAN do host Unraid, o endereço em que abres a interface web.
3. Se a rede do container e o host estiverem em VLANs diferentes, permite a rota no teu router ou firewall: da VLAN do container para o IP do host, na porta SSH, por TCP.
4. Se o IP LAN do host continuar inalcançável, usa o IP shim do host na sub-rede do container. Encontra-o no host com `ip -4 addr show | grep -B2 <container-subnet>`.

### Verificar se o host responde {#verify-reachability}

Corre isto no terminal do Unraid, com o teu IP e porta:

```sh
docker exec BombVault timeout 6 bash -c 'echo > /dev/tcp/192.168.1.10/22' && echo OPEN || echo UNREACHABLE
```

`OPEN` significa que o caminho funciona. `UNREACHABLE` significa que a rede do passo 3 precisa de ser corrigida antes de mais nada.

## Passo 4: definir as variáveis e testar {#step-4}

1. No Unraid, abre **Docker**, clica no BombVault, escolhe **Edit**, define **Host SSH: Address** (e **Host SSH: Port** se não for 22) e carrega em **Apply**. Se as variáveis estiverem em falta, importa o template outra vez, porque o Unraid mantém a configuração guardada de um container existente.
2. No BombVault, abre **Definições, Integrações, SSH do anfitrião** e carrega em **Testar ligação**. Deve ficar verde.

A mesma chamada a partir do terminal lista as tuas VMs:

```sh
docker exec BombVault virsh -c "qemu+ssh://root@192.168.1.10:22/system?keyfile=/config/ssh/id_ed25519&known_hosts=/config/ssh/known_hosts&known_hosts_verify=auto" list --all
```

## Passo 5: o método por VM {#per-vm-method}

Cada VM tem um **Método** no cartão **VM** do seu separador Definições (abre a VM em **Backups**):

- **Ordenado (encerramento)** é o predefinido. Desliga a VM, faz backup dos discos e volta a arrancá-la, por isso o backup é sempre consistente. Espera até dez minutos que o convidado se desligue e nunca o força a desligar. Um convidado que ainda esteja a correr depois disso fica em paz, e esse backup falha com o motivo.
- **Instantâneo a quente** faz backup de uma VM em execução através de um instantâneo externo e de `blockcommit`. Precisa do agente convidado qemu na VM, para um instantâneo em repouso, e do disco em `/mnt/cache` ou `/mnt/diskX`, não em `/mnt/user`. Numa VM desligada, o a quente recorre sozinho ao ordenado, e quando o instantâneo não pode ser criado, recorre também.

## Só blocos alterados {#changed-blocks}

Uma VM com discos qcow2 pode ser copiada lendo apenas o que mudou. Liga **Só blocos alterados** no mesmo cartão, e o BombVault usa a API de backup do libvirt em vez do método acima:

- Inicia um trabalho de backup em modo pull (`virsh backup-begin`) que cria no mesmo instante um checkpoint chamado `bombvault-<hora>`, e lê os discos pela ligação SSH através de um socket Unix reencaminhado. Quando o agente convidado responde, os sistemas de ficheiros do convidado só ficam congelados enquanto o trabalho arranca. Um descongelamento que falhe é tentado outra vez durante cerca de quinze segundos, e se o convidado continuar sem descongelar, o BombVault envia uma notificação.
- A primeira execução lê todos os blocos alocados. As seguintes leem só os blocos que o bitmap do checkpoint marca como escritos desde o backup anterior.
- Cada disco é guardado como uma pasta de segmentos de tamanho fixo com o que o convidado vê. Os segmentos inalterados são aproveitados do instantâneo anterior sem serem lidos, por isso cada instantâneo está completo e restaura-se sozinho.
- Uma restauração reconstrói o disco a partir dos seus segmentos e converte-o de volta para qcow2 com o `qemu-img`. O disco tem o mesmo conteúdo que no momento do backup, mas o ficheiro é um ficheiro qcow2 novo, e uma cadeia de backing volta achatada.
- A verificação antes dessa restauração lista cada disco que reconstrói e conta-o no tamanho total do convidado, duas vezes para o qcow2, já que a imagem em bruto e a sua cópia convertida ficam lado a lado até o ficheiro antigo ser substituído. É o máximo que um disco pode ocupar, por isso uma falta de espaço por esta conta deixa a linha do espaço cinzenta em vez de bloquear. A verificação de restauro depois do primeiro backup, e **Verificar restauro**, confirmam também que o instantâneo tem todos os segmentos de todos os discos no tamanho que deviam ter.
- Entre execuções a VM não tem nenhum checkpoint registado no libvirt. O BombVault guarda a definição do checkpoint `bombvault-` mais recente em `/config/vm-checkpoints` e remove só o registo dele no libvirt (`virsh checkpoint-delete --metadata`). O seu bitmap fica no ficheiro qcow2 e continua a contar escritas, e a execução seguinte volta a pôr o checkpoint (`virsh checkpoint-create --redefine`) antes de começar. O libvirt recusa anular a definição de uma VM que tenha checkpoints, por isso é isto que permite que o **Remove VM** do Unraid e uma restauração funcionem como de costume.
- Um trabalho de backup que o BombVault deixou a correr, porque parou a meio de um backup, é terminado quando o BombVault volta a arrancar, juntamente com os checkpoints que essa execução deixou registados.
- Desligar o interruptor, tirar a VM do agendamento ou eliminar os seus backups também termina esse trabalho, e apaga também o bitmap enquanto a VM corre. Numa VM desligada, o libvirt não o consegue apagar, e ele fica na imagem sem uso.
- Os checkpoints de outras ferramentas são deixados em paz. Uma VM que tenha alguns não pode ser restaurada até serem apagados, e a restauração pára antes de tocar na VM.

O interruptor recorre ao método habitual sozinho. Um checkpoint em falta ou avariado, um disco que mudou de tamanho, ou um instantâneo mais recente que não seja de blocos alterados significa que o disco inteiro é lido mais uma vez. Uma VM desligada, um disco raw ou zvol, nenhuma ligação SSH ao host, um libvirt sem a API de backup, ou pouco espaço livre em `/config` significam o backup habitual com o método acima. O cartão diz qual destes a última execução usou e porquê.

Requisitos: libvirt com a API de backup (testado com libvirt 12.2 e QEMU 10.2 no Unraid 7.3), sshd com `AllowStreamLocalForwarding yes` (o predefinido, também no Unraid, que desliga o encaminhamento TCP) e discos qcow2. O libvirt recusa checkpoints em discos raw, e o Unraid cria vdisks raw por predefinição. A VM continua a correr durante o backup, como com o a quente, e os segmentos à espera do restic ocupam até 4 GiB em `/config`.

## VMs com instantâneos do Unraid {#unraid-snapshots}

Um instantâneo que o Gestor de VMs do Unraid tira transforma um disco qcow2 numa cadeia: a imagem original em baixo e, por cima dela, um ficheiro por instantâneo, cada um com apenas o que mudou desde o de baixo. A VM escreve no ficheiro mais recente. Um disco com dois instantâneos são três ficheiros.

O BombVault faz backup de uma VM destas por inteiro:

- **Todos os ficheiros da cadeia.** Segue a cadeia de backing de cada disco até ao fim e faz backup de todos os ficheiros dela, juntamente com a lista de instantâneos do Unraid (`/etc/libvirt/qemu/snapshotdb/<vm>/snapshots.db`, lida por SSH). Uma cadeia que não se consegue seguir até ao fim, por exemplo porque falta um ficheiro a meio, faz o backup falhar e nomeia o ficheiro.
- **Um backup a quente mantém os instantâneos.** O **Instantâneo a quente** põe uma sobreposição temporária por cima enquanto lê o disco e junta-a de volta depois, só à imagem mesmo por baixo. A cadeia é a mesma depois do backup e antes dele. Uma VM sem instantâneos comporta-se como sempre.
- **Uma restauração traz a cadeia de volta.** Uma restauração no lugar escreve cada ficheiro de volta onde estava e repõe a lista de instantâneos do Unraid, por isso o Gestor de VMs volta a listar os instantâneos. Uma restauração ao lado do original escreve os ficheiros na pasta da cópia e aponta os discos restaurados para os ficheiros restaurados, por isso a cópia nunca lê das imagens do original. A lista de instantâneos do Unraid só volta com uma restauração no lugar, porque nomeia os ficheiros onde estavam quando foram copiados. Sem ela a cópia arranca na mesma, e o Gestor de VMs apenas não lista instantâneos para ela.
- **Só blocos alterados** lê o que o convidado vê, por isso a sua restauração volta como um só ficheiro qcow2 achatado, sem os instantâneos.

### Exemplo: uma VM com dois instantâneos {#example-unraid-snapshots}

A VM `homeassistant` tem um disco, `vdisk1.qcow2`, e dois instantâneos que tiraste no Gestor de VMs antes de duas atualizações. O seu disco é agora uma cadeia de três ficheiros. Corre sempre, por isso usa **Instantâneo a quente**.

1. Abre **Backups**, depois `homeassistant`, depois o seu separador **Definições**. No cartão **VM**, confirma que o **Método** é **Instantâneo a quente** e que **Só blocos alterados** está desligado.
2. Carrega em **Fazer backup agora** no cabeçalho da página.
3. Quando a execução termina, a sua linha em **Execuções recentes** no separador **Verificação** está verde. A VM continuou a correr o tempo todo, e o backup guarda os três ficheiros do disco e a lista de instantâneos do Unraid.
4. No Gestor de VMs, os dois instantâneos continuam lá, e o disco continua a ler através dos mesmos três ficheiros.

Um mês depois uma atualização corre mal e queres o estado de ontem ao lado do estragado, para comparar:

1. Abre **Restauração**, escolhe **VMs** e clica em `homeassistant`.
2. Mantém o backup recomendado da noite passada e escolhe **Ao lado do original** em **Para onde**.
3. Carrega em **Restaurar**. Os três ficheiros ficam numa pasta com o nome da VM mais `-restored`, e o disco restaurado aponta para os ficheiros restaurados. Nada muda na VM em execução.

Para repor antes a VM inteira, escolhe **Substituir o original** com **Fazer antes um backup do estado atual** ligado. O BombVault faz backup do estado estragado, desliga a VM, escreve de volta os três ficheiros e a lista de instantâneos do Unraid, e volta a arrancá-la.

## TrueNAS SCALE {#truenas-scale}

Os passos acima estão escritos para o Unraid, mas a mesma ligação `qemu+ssh://` funciona com qualquer libvirtd que o container consiga alcançar, o TrueNAS SCALE incluído, que tem VMs em libvirt de novo desde a 25.04.2. No TrueNAS, a string de ligação precisa de uma substituição e a chave vai para o root nas definições de utilizador do próprio TrueNAS. Mais três coisas mudam o que um backup guarda: discos em zvols, o estado do vTPM e os nomes que o TrueNAS dá às suas VMs.

### 1. O socket do libvirt {#truenas-socket}

O libvirtd do TrueNAS não escuta onde a string de ligação construída a partir dos campos **Host SSH** espera. Define antes `LIBVIRT_URI`, com o parâmetro extra `?socket=`:

```sh
LIBVIRT_URI=qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock
```

O BombVault usa o `LIBVIRT_URI` exatamente como está escrito, e `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` e `LIBVIRT_SSH_PORT` não são precisos. Para cada uma delas que não esteja definida, o BombVault toma o host, o utilizador e a porta do URI também para os seus próprios comandos SSH, por isso a cópia da NVRAM e os backups de conjuntos de dados ZFS chegam à mesma máquina que o `virsh`. Uma variável que definas tu ganha sempre, e o registo de arranque nomeia o alvo que tirou do URI. No TrueNAS, um utilizador que não seja o root precisa também de `zfs allow <user> snapshot,destroy,mount <dataset>` para os backups de conjuntos de dados ZFS, vê [Conjuntos de dados ZFS](zfs-datasets.md#truenas).

Põe o endereço LAN do TrueNAS no URI. Sem `LIBVIRT_URI` nem `LIBVIRT_HOST` definidos, o BombVault toma o gateway predefinido da aplicação como host, porque `host.docker.internal` não resolve numa aplicação do TrueNAS, e o registo de arranque nomeia o endereço que escolheu. Isso chega para os backups de conjuntos de dados ZFS. Os backups de VMs continuam a precisar do URI para o socket.

A aplicação precisa da capability `SETPCAP` para os backups de VMs, a par de `CHOWN`, `DAC_OVERRIDE`, `FOWNER`, `SETGID` e `SETUID`. Sem ela o libvirt pára com "cannot apply process capabilities". O YAML em [TrueNAS SCALE](getting-started.md#truenas) define todas, juntamente com o `SYS_CHROOT` de que o instantâneo em vez de parar precisa.

No TrueNAS SCALE 25.10.0 o caminho do socket foi verificado em hardware real: o `qemu:///system` de origem falha com `Failed to connect socket to '/var/run/libvirt/libvirt-sock': No such file or directory`, e a mesma chamada com `?socket=/run/truenas_libvirt/libvirt-sock` lista os domínios.

### 2. A chave do BombVault no TrueNAS {#truenas-key}

O passo 2 acima escreve na unidade flash do Unraid e não se aplica aqui. No TrueNAS:

1. Liga o serviço SSH em **System, Services**.
2. Em **Credentials, Users**, edita o **root** e cola a chave pública do BombVault de **Definições, Integrações, SSH do anfitrião** como a sua chave pública SSH. O TrueNAS guarda-a entre reinícios e atualizações.

As instalações novas do TrueNAS 24.10 e mais recentes não dão palavra-passe ao root, e o início de sessão por chave não precisa de nenhuma. Se preferires não usar o root, autoriza a chave para uma conta de administrador com sudo e põe o nome dela no lugar de `root` no `LIBVIRT_URI`. Os backups de conjuntos de dados ZFS precisam então de `zfs allow`, como descrito acima.

Uma réplica ZFS para um segundo servidor entra com uma chave própria, que a janela da réplica mostra. A chave SSH do anfitrião não autoriza uma réplica, e a chave da réplica não autoriza backups de VMs.

!!! warning "O TrueNAS não vê as alterações feitas com o virsh"
    O middleware do TrueNAS, a camada por trás dos seus ecrãs de VMs, deve ser a única forma de alterar as suas VMs. Um comando `virsh` por esta ligação SSH, que é o que o backup e a restauração de VMs do BombVault correm, passa ao lado dele, e o TrueNAS pode mais tarde reconciliar ou sobrescrever um estado que não criou. Isto vem do comportamento documentado do TrueNAS e não foi visto numa máquina real. É o mesmo tipo de risco que o Gestor de VMs do Unraid reconciliar alterações feitas fora dele, só que mais agudo no TrueNAS.

### 3. Discos em zvols {#truenas-zvols}

Os discos de VMs do TrueNAS são muitas vezes zvols, dispositivos de bloco ZFS em `/dev/zvol/<pool>/<dataset>`, e não ficheiros qcow2. O restic não consegue fazer backup de um dispositivo de bloco pelo seu caminho como faz com um ficheiro, por isso o BombVault reconhece um disco zvol no XML do domínio (`<source dev="...">` em vez de `<source file="...">`) e trata dele pela mesma ligação SSH. O container do BombVault não precisa de ferramentas ZFS.

Um backup de um zvol:

1. `zfs snapshot <dataset>@bombvault-vm-<hora>` fixa um ponto no tempo, também com a VM a correr.
2. `zfs clone -o volmode=dev -o readonly=on` torna esse instantâneo legível como dispositivo. O `volmode=dev` impede o host de analisar as partições do convidado.
3. O `dd` lê o dispositivo, e a imagem raw entra no restic como um só ficheiro, `/vm-disks/<dataset>.img`, com a etiqueta `vm:<nome>:zvol:<dev>`. O restic corta-a em pedaços por conteúdo, por isso os blocos que não mudaram desde o último backup não são guardados outra vez.
4. O clone e o instantâneo são destruídos depois, quer o backup tenha funcionado ou não. Se o BombVault parou a meio de um backup, o backup seguinte desse zvol remove primeiro o instantâneo `bombvault-vm-<hora>` e o seu clone `<dataset>-bombvault-vm-<hora>`. Deixa em paz todos os outros instantâneos, e mantém aquele de que algo mais foi clonado. O registo nomeia o que removeu ou manteve.

O BombVault não usa `zfs send` para isto. Cada registo de um fluxo de send leva o GUID do instantâneo e uma soma de verificação corrente, por isso o restic não encontraria nada do último backup para reaproveitar e guardaria o disco inteiro de cada vez. Medido no TrueNAS SCALE 25.10 com um zvol de 512 MiB com 200 MiB de dados: o primeiro backup acrescentou 211 MB, um segundo sem nada alterado 52 KB, e um depois de 1 MiB ter mudado 3,6 MB. O mesmo zvol por `zfs send` acrescentou 214 MB em cada execução.

Cada execução escreve também um pequeno instantâneo com a etiqueta `vm:<nome>` com o XML do domínio, quando todos os discos estão dentro. Uma VM cujos discos são todos zvols não tem instantâneo de ficheiros, e este é o que lista a execução sob a VM e a torna restaurável.

Uma restauração escreve cada zvol de volta no lugar, tal como um disco de ficheiro é escrito de volta no seu ficheiro, por isso a VM restaurada arranca com os dados restaurados:

1. Antes de se tocar em qualquer coisa, o BombVault verifica que a execução tem uma imagem de cada disco zvol. Uma execução a que falte uma é recusada e a VM fica como está.
2. Um zvol existente recebe primeiro um instantâneo, `<dataset>@bombvault-before-restore-<hora>`, com a hora em UTC como todos os nomes de instantâneos que o BombVault faz, por isso pode estar horas desfasada do relógio local. O registo nomeia-o, e um `zfs rollback` para ele desfaz a restauração enquanto for o instantâneo mais recente do zvol. Quando existe um mais novo, como o da restauração seguinte, o `zfs rollback -r` faz isso e destrói os instantâneos mais novos. Fica até o removeres com `zfs destroy` quando a VM restaurada funcionar.
3. Um zvol que já não existe é criado no tamanho que tinha quando foi copiado, e um existente de outro tamanho volta a esse tamanho.
4. O `dd` escreve a imagem em `/dev/zvol/<dataset>`. Com a compressão ligada, a predefinição do TrueNAS, as partes vazias da imagem voltam como buracos e não ocupam espaço.

**Uma restauração para outra instância precisa de um pool de destino.** Move os discos de ficheiros para uma pasta de destino, mas uma pasta não nomeia nenhum pool ZFS, por isso sem um o BombVault recusa a restauração antes de começar. Passa o pool no parâmetro `zvolPool` de `POST /api/foreign/restore`: cada zvol mantém o seu caminho abaixo do pool (`tank/vms/win10/disk0` para `flashpool` passa a `flashpool/vms/win10/disk0`), e o XML do domínio aponta para ele. A janela **De uma pasta de backups antiga** na página Restauração não tem campo para isto, por isso só se chega lá com uma chamada direta à API. Uma restauração na mesma instância escreve cada zvol de volta em si mesmo.

**A exportação como ficheiros deixa os zvols de fora.** Uma VM cujos discos são todos zvols é recusada com esse motivo, por isso restaura-a antes a partir de um backup. Para uma VM que tenha também discos de ficheiros, a exportação guarda os discos de ficheiros e uma nota que nomeia os zvols que deixou de fora.

Isto foi experimentado no TrueNAS SCALE 25.10 com ZFS 2.3.4: três backups de um zvol de teste, uma restauração no lugar depois de o zvol ter sido sobrescrito e aumentado, e uma restauração para um zvol que tinha sido apagado. Cada restauração leu de volta com o mesmo SHA-256 da origem, o zvol voltou a ter o seu tamanho antigo, e nenhum instantâneo ou clone ficou para trás. Uma VM inteira cujo único disco é um zvol foi copiada a correr, com Ordenado e com a quente, e desligada, e uma restauração de VM inteira (anular a definição, escrever os zvols, definir, arrancar) correu também aí: o zvol leu de volta com a soma de verificação da origem e a VM arrancou.

Os discos de ficheiros, como o Unraid usa, não são afetados por nada disto.

### 4. Estado do vTPM {#truenas-vtpm}

Uma VM pode ter um TPM virtual, de que o Secure Boot e os convidados Windows 11 precisam. O seu estado vive fora do disco e da NVRAM, por isso um backup só desses dois traria o convidado de volta com um TPM novo e vazio.

- **Um TPM de passthrough**, um chip real passado ao convidado, nomeia o seu caminho no XML do domínio. O BombVault guarda o seu estado junto à NVRAM por SSH e escreve-o de volta antes do `virsh define`, em melhor esforço como a NVRAM: uma leitura ou escrita falhada fica registada e não faz o backup nem a restauração falhar.
- **Um vTPM emulado**, do tipo que o TrueNAS configura para convidados Windows 11 e Secure Boot, não publica nenhum caminho de estado no XML do domínio. O BombVault comunica então "no TPM path found" em vez de adivinhar um caminho. Tem à mão a chave de recuperação desse convidado, como a sua chave BitLocker, antes de o restaurares.
- **Não podes anexar tu próprio um TPM a uma VM do TrueNAS.** A API de dispositivos de VM do TrueNAS SCALE 25.10 só aceita `CDROM`, `DISPLAY`, `NIC`, `PCI`, `RAW`, `DISK` e `USB`, e recusa `TPM` como tipo desconhecido.

O tratamento do TPM só foi testado com testes unitários e não correu contra um convidado TrueNAS real com um vTPM. Uma VM sem TPM, como no Unraid, copia e restaura-se exatamente como antes.

A NVRAM é captada como de costume. O TrueNAS guarda-a por VM em `/var/db/system/vm/nvram/<id>_<name>_VARS.fd`, e o XML do domínio nomeia esse caminho.

### 5. Nomes de VMs no TrueNAS {#truenas-names}

No Unraid, o nome libvirt de uma VM é o nome que escolheste. O TrueNAS inventa o nome libvirt, e mudou a forma duas vezes:

| Versão do TrueNAS | Nome libvirt | Onde vive o teu nome |
|---|---|---|
| 25.10 "Goldeye" | `{id}_{name}`, por exemplo `1_debian` | no nome libvirt, atrás do `{id}_` |
| 26 | o UUID da VM, por exemplo `550e8400-e29b-41d4-a716-446655440000` | só no `<title>` do XML do domínio |

Num host que o BombVault deteta como TrueNAS, ou que `PLATFORM=truenas` nomeia, a lista Backups mostra o teu nome: `debian` em vez de `1_debian` na 25.10, e o `<title>` em vez do UUID na 26, ou o UUID quando não há título. Isto é só o que a lista mostra. Cada chamada `virsh` e as definições por VM continuam a usar o nome libvirt, por isso o backup e a restauração atuam sempre na VM certa. No TrueNAS 26 isso significa que uma VM que o TrueNAS cria outra vez com um UUID novo aparece como uma entrada nova sem as suas definições, mesmo que o nome pareça o mesmo. No Unraid nada muda, também para uma VM a que tenhas chamado `10_Windows`.

O esquema da 25.10 foi visto numa máquina TrueNAS SCALE 25.10 real. O esquema do TrueNAS 26 vem da documentação do TrueNAS e só foi testado com testes unitários.

## Resolução de problemas {#troubleshooting}

| Sintoma | Causa e solução |
|---|---|
| O **Testar ligação** fica pendurado e depois falha | O host não se consegue alcançar. Verifica o passo 3 (rede, VLAN, firewall) e **Host SSH: Address** e **Port**. |
| `Permission denied (publickey)` embora a chave esteja em `authorized_keys` | O `StrictModes` do sshd rejeita o ficheiro de chaves porque uma pasta acima tem o dono ou o modo errados. É comum quando `/root/.ssh` é uma ligação para a unidade flash FAT (`/boot/config/ssh/...`), que é gravável por todos. O registo do host mostra `Authentication refused: bad ownership or modes for directory ...`. Acrescenta `StrictModes no` à configuração do sshd do host, na secção global, antes de qualquer bloco `Match`, e corre `/etc/rc.d/rc.sshd restart`. O Unraid escreve `/etc/ssh/sshd_config` de novo em cada arranque, por isso torna-o persistente como descrito abaixo. |
| `Permission denied (publickey)` e a chave está mesmo em falta | Repete o passo 2 e confirma que o SSH está ligado. |
| `Host key verification failed` | Corre `docker exec BombVault rm -f /config/ssh/known_hosts` e testa outra vez. |
| A verificação do passo 3 diz `UNREACHABLE` | O container não consegue alcançar a porta SSH do host. Vê o passo 3: acesso do host a redes personalizadas, encaminhamento de VLAN, ou o IP shim ou LAN. |
| As variáveis faltam em **Edit** | Importa o template outra vez. Um container existente mantém a sua antiga configuração guardada. |
| Uma VM com instantâneos do Unraid falha e nomeia um ficheiro | A cadeia de backing do disco não se consegue seguir até ao fim. Confirma no Gestor de VMs que todos os instantâneos desse disco ainda lá estão, e que o ficheiro nomeado existe abaixo de `/mnt`. |

Há mais sintomas de VMs em [Resolução de problemas](troubleshooting.md#vm-backup-will-not-connect).

## Manter tudo depois de reinícios {#persistence}

O Unraid escreve `/etc/ssh/sshd_config` de novo em cada arranque, por isso as alterações do lado do host têm de ser refeitas no arranque:

- A **porta SSH** fica em `/boot/config/ident.cfg`, definida pela interface web do Unraid.
- O **`StrictModes no`**, se precisaste dele acima, não se mantém. Aplica-o no arranque a partir de `/boot/config/go`:

  ```sh
  # /boot/config/go: keep SSH key auth working with flash-based authorized_keys
  ( for i in $(seq 1 30); do [ -f /etc/ssh/sshd_config ] && break; sleep 2; done
    grep -q '^StrictModes no' /etc/ssh/sshd_config || \
      sed -i '0,/^Match /s/^Match /StrictModes no\n&/' /etc/ssh/sshd_config
    # SIGHUP reloads sshd's config in place: it keeps the listener + open sessions
    # and avoids the "killing listener process / Restarting SSH server daemon"
    # warning that `rc.sshd restart` prints at boot. Only reload if sshd is already
    # up; if it starts after this runs, it reads the edited config on its own.
    pidof sshd >/dev/null && killall -HUP sshd ) &
  ```

- A chave SSH do próprio BombVault e o `~/.ssh/config` vivem em `/config` (appdata) e são refeitos em cada arranque, por isso sobrevivem a reinícios do container e do host.

## Segurança {#security}

A chave autorizada dá root no host, o mesmo nível de confiança que o socket Docker que o BombVault já usa. Mantém o BombVault numa rede de confiança. A chave é ed25519, guardada com o modo `0600` no appdata, e a chave do host fica fixada. Cada comando para o host é passado como argumentos separados, nunca por um shell remoto, e a ligação SSH usa `BatchMode` e um limite de tempo de ligação.
