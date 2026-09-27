# Conjuntos de dados ZFS

A página **ZFS** faz backup de conjuntos de dados ZFS. Um elemento é um conjunto de dados juntamente com todos os conjuntos de dados abaixo dele. Para cada backup, o BombVault tira um único instantâneo ZFS de toda a árvore, pelo que cada conjunto de dados nela é capturado no mesmo instante. Depois lê os ficheiros de cada conjunto de dados a partir desse instantâneo, guarda-os com o restic da mesma forma que guarda uma pasta e remove o instantâneo logo a seguir. Os backups são deduplicados, pode navegar em cada um deles e é possível restaurar ficheiros individuais.

O BombVault nunca usa `zfs send` para conjuntos de dados, nunca reverte um conjunto de dados e nunca destrói nenhum.

## Requisitos {#requirements}

- **A ligação SSH a este servidor.** Os conjuntos de dados ZFS usam a mesma chave, o mesmo anfitrião e o mesmo utilizador que os backups de VM. Se os backups de VM já funcionam, isto também funciona. Caso contrário, siga o [guia de backup de VM por SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) no GitHub. Os campos do modelo chamam-se **Host SSH: Address**, **Host SSH: Port** e **Host SSH: User**.
- **O comando `zfs` nesse anfitrião.** O Unraid 6.12 e posteriores e o TrueNAS SCALE têm-no.
- **Host Data mapeado como `/mnt` com o Access Mode Read/Write - Slave.** É o valor predefinido do modelo. O instantâneo de um conjunto de dados só aparece dentro da pasta `.zfs/snapshot` do conjunto de dados depois de o BombVault ter arrancado, por isso o contentor tem de receber as montagens que o anfitrião faz mais tarde.
- **Os conjuntos de dados montados abaixo de `/mnt`.** No Unraid, os pools ficam em `/mnt/<pool>`, por isso isso já acontece.

Ative o domínio em **Definições, Geral** (Conjuntos de dados ZFS). A página ZFS mostra então um cartão **Ligação a este servidor**. Testa a ligação SSH, indica o utilizador e o anfitrião a que se liga e diz o que falta quando falta alguma coisa. A verificação de integração com o anfitrião (`/spike`) mostra o mesmo resultado.

## Elementos e conjuntos de dados filhos {#items-and-children}

Abra **Adicionar conjuntos de dados** na página ZFS. A lista vem do servidor. Escolha o conjunto de dados que está mais acima naquilo que quer guardar, por exemplo `cache/appdata`, e o elemento abrange-o a ele e a todos os conjuntos de dados abaixo dele.

- **Os novos conjuntos de dados filhos entram sozinhos.** Um conjunto de dados criado mais tarde abaixo do elemento é guardado na execução seguinte, que o indica como novo. O primeiro backup lê-o por inteiro uma vez; depois só são lidas as alterações.
- **Pode deixar de fora filhos individuais.** Desative um filho nas definições do elemento e ele fica de fora juntamente com tudo o que está abaixo. Um filho excluído que já não existe no servidor é assinalado como tal e pode ser retirado da lista.
- **Os filhos que não podem ser lidos são ignorados, nunca em silêncio.** A execução enumera-os, o elemento mostra quantos foram ignorados e o cartão de cobertura do painel conta cada um como não protegido. A execução guarda na mesma todo o resto e não falha por causa de um filho ignorado. Os motivos estão na [tabela de códigos de motivo](#reason-codes): um conjunto de dados não montado, com `canmount=off`, com um ponto de montagem `legacy` ou sem ele, uma chave de encriptação não carregada, o acesso a instantâneos desligado ou um ponto de montagem que o BombVault não vê.
- **Um conjunto de dados ignorado não arrasta os filhos consigo.** Um conjunto de dados com `canmount=off` que só contém outros conjuntos de dados é ignorado (mostrado como "só estrutura") e os seus filhos montados são guardados. Um conjunto de dados encriptado cuja chave não está carregada é ignorado juntamente com os filhos que partilham a sua chave.
- **Os filhos que são discos de VM ou dados do sistema começam desativados** na caixa de diálogo de adição, com o motivo junto ao interruptor. Adicionar um pool inteiro pede uma confirmação que enumera o que ele contém.

### Volumes {#volumes}

Um volume (zvol) contém um disco virtual em vez de ficheiros, e a página ZFS nunca o guarda.

- Um volume usado por uma VM é guardado com essa VM na página **VMs**.
- Um volume que nenhuma VM usa (um extent iSCSI, um disco que desligou) **não é guardado pelo BombVault**. A caixa de diálogo de adição e a página ZFS contam esses volumes e dizem-no. Uma versão futura vai guardá-los.

Os volumes dentro da árvore de um elemento são ignorados e indicados em cada execução.

### O armazenamento do Docker {#docker-storage}

Com o controlador de armazenamento ZFS do Docker, cada camada de imagem é um conjunto de dados com um ponto de montagem `legacy`. A caixa de diálogo de adição agrupa-os numa linha por pai. Uma árvore com mais de 20 desses conjuntos de dados não pode tornar-se um elemento: enquanto existir um instantâneo dela, o Docker não consegue remover camadas de imagem. Adicione antes os conjuntos de dados abaixo dela, por exemplo `appdata`.

### Os elementos nunca se sobrepõem {#overlap}

Um conjunto de dados só pode pertencer a um elemento. O BombVault recusa um elemento novo que fique dentro de um existente ou que contivesse um. Para juntar vários elementos filhos num elemento pai, elimine primeiro os elementos filhos escolhendo manter os seus backups e depois adicione o pai. Cada conjunto de dados mantém o seu histórico com o próprio nome, por isso o backup seguinte continua onde os elementos antigos ficaram e não volta a ler tudo.

## Parar contentores e correr comandos à volta do instantâneo {#consistency}

Um instantâneo de uma base de dados em execução é como uma falha de energia súbita: a base de dados normalmente recupera, mas tem de o fazer. Cada elemento pode fazer duas coisas quanto a isso, e ambas cobrem apenas o instante do instantâneo, não o backup inteiro.

- **Parar estes contentores para o instantâneo.** O BombVault para os contentores da lista, tira o instantâneo e volta a arrancá-los de imediato. Os contentores de um mesmo nível de dependências param em paralelo, primeiro os dependentes, por isso a janela inteira dura normalmente alguns segundos; a execução mostra quanto durou. O backup lê depois o instantâneo congelado enquanto as aplicações já estão outra vez a correr. Só são parados os contentores que estavam a correr.
- **Um comando antes e depois do instantâneo.** Corre dentro de um contentor à sua escolha, por exemplo para exportar uma base de dados para o conjunto de dados mesmo antes do instantâneo, sem parar nada. Se o comando antes do instantâneo falhar, o backup falha e não é tirado nenhum instantâneo. Um comando depois do instantâneo que falhe é mostrado na execução mas não faz falhar o backup.

O que acontece quando algo corre mal:

- Se um contentor não puder ser parado, o BombVault arranca os que já parou e o backup falha, indicando o contentor. Nunca recorre a um instantâneo de aplicações em execução.
- A paragem espera que termine um backup de contentor em curso (até 30 minutos numa execução manual, até ao limite de tempo do backup numa agendada), para que os dois nunca parem e arranquem o mesmo contentor ao mesmo tempo.
- Antes de o primeiro contentor parar, o BombVault anota quais para. Se o BombVault for terminado dentro da janela, volta a arrancar esses contentores no seu arranque seguinte, envia uma notificação e o elemento mostra uma nota vermelha para cada contentor que não conseguiu arrancar.

As exportações automáticas de bases de dados (ver [Funcionalidades](features.md)) correm com o backup próprio de um contentor na página **Containers**, não com um elemento ZFS. Uma base de dados cujo contentor só é guardado através do seu conjunto de dados não recebe exportação, por isso dê-lhe aqui um comando.

Um contentor pode estar nesta lista e na página **Containers** ao mesmo tempo. Os seus dados ficam então guardados duas vezes, em dois repositórios, e o **Backup total** para-o duas vezes. O elemento avisa disso.

## Restaurar {#restore}

Abra **Backups** no elemento, escolha o backup e depois o conjunto de dados. Por predefinição é o conjunto de dados de topo do elemento.

- **Restaurar dentro do conjunto de dados.** Os ficheiros do backup são escritos no ponto de montagem do conjunto de dados. Os ficheiros com o mesmo nome são substituídos, os outros ficam. O conjunto de dados nunca é revertido nem substituído. O BombVault verifica que o conjunto de dados está montado, visível e gravável, uma vez antes de começar e de novo mesmo antes de escrever. Onde um conjunto de dados filho estiver montado lá dentro, nada é escrito: o filho mantém os seus ficheiros, o dono e as permissões, e é restaurado a partir do seu próprio backup.
- **Restaurar para uma pasta.** Escolha uma pasta abaixo de `/mnt`. O BombVault verifica que a pasta está num pool ou partilha montados e que há espaço livre suficiente. Funciona sem a ligação SSH e para conjuntos de dados que já não existem.
- **Escolher ficheiros** (avançado): escrever de volta no conjunto de dados apenas os ficheiros e pastas que escolher.
- **Todos os conjuntos de dados desta cópia** (avançado): cada conjunto de dados da árvore na sua própria subpasta da pasta que escolher. Os conjuntos de dados ignorados nesse backup são indicados.
- **A partir de outro servidor:** a página **Recuperação** restaura a partir do repositório de outro BombVault, sempre para uma pasta: todos os conjuntos de dados de um backup, cada um na sua própria subpasta, ou um conjunto de dados da árvore, inteiro ou ficheiros escolhidos.

A lista de contentores a parar do elemento também é oferecida para um restauro dentro do conjunto de dados. Esses contentores ficam parados durante todo o restauro, e entretanto os backups de contentores esperam.

### O instantâneo de segurança {#safety-snapshot}

Antes de escrever num conjunto de dados, o BombVault tira um instantâneo ZFS apenas desse conjunto de dados, com o nome `bombvault-prerestore-<hora>`. Está ligado por predefinição; desligá-lo exige uma segunda confirmação. Se não for possível tirar o instantâneo, nada é restaurado.

O BombVault nunca apaga sozinho um instantâneo de segurança. O elemento enumera-os com a idade e o tamanho, cada um com a ação **Excluir**, e avisa quando o mais antigo tem mais de 30 dias, porque retém no pool dados apagados e alterados.

Para voltar atrás depois de um restauro, copie ficheiros individuais de `.zfs/snapshot/bombvault-prerestore-<hora>` dentro do conjunto de dados. `zfs rollback <dataset>@bombvault-prerestore-<hora>` só funciona enquanto for o instantâneo mais recente desse conjunto de dados. `zfs rollback -r` apaga todos os instantâneos mais recentes, incluindo os automáticos.

### Restaurar como conjunto de dados novo {#new-dataset}

O BombVault não cria conjuntos de dados. Crie-o no servidor com as propriedades que quiser e depois restaure para uma pasta que seja o seu ponto de montagem:

```
zfs create -o compression=lz4 cache/appdata-restored
```

e no BombVault restaure para a pasta `cache/appdata-restored` abaixo de `/mnt`.

## O que está no backup {#contents}

No backup: os ficheiros e pastas de cada conjunto de dados guardado, com o dono, as permissões, as datas e os atributos estendidos tal como o restic os guarda.

Não está no backup:

- as propriedades ZFS dos conjuntos de dados (compression, recordsize, quota, mountpoint e as restantes);
- o dono e as permissões da própria pasta de topo de cada conjunto de dados (tudo o que está abaixo dela está incluído). Um restauro dentro do conjunto de dados deixa a pasta de topo existente como está; um restauro para uma pasta cria-a com as permissões `0755`;
- os instantâneos ZFS existentes;
- os filhos que foram ignorados ou deixados de fora;
- os volumes.

Para restaurar num pool novo, crie primeiro os conjuntos de dados com as propriedades que quiser. Ainda não foi verificado se as ACL NFSv4, tal como o TrueNAS as usa em conjuntos de dados SMB, voltam como espera, por isso teste um restauro com os seus próprios dados antes de confiar nelas.

## Conjuntos de dados encriptados {#encryption}

Um conjunto de dados encriptado só é guardado enquanto a sua chave estiver carregada. Caso contrário, é ignorado com um aviso; carregue a chave com `zfs load-key` e monte o conjunto de dados. O BombVault lê os dados desencriptados e guarda-os no repositório do restic, que é encriptado. Se desligou a encriptação no BombVault, esse repositório não o é.

## Instantâneos que sobraram {#leftover-snapshots}

O instantâneo de um backup chama-se `<dataset>@bombvault-<14 dígitos>`, por exemplo `cache/appdata@bombvault-20260924021500` (UTC). O BombVault remove-o logo após o backup. Se isso falhar, por exemplo porque o conjunto de dados está ocupado ou o BombVault foi parado, o BombVault remove-o:

- antes do backup seguinte desse elemento,
- quando o BombVault arranca, para cada elemento, mesmo com o domínio desligado,
- quando elimina o elemento,
- quando carrega em **Remover agora** no elemento, que mostra quantos restam.

Só são removidos os nomes que correspondem exatamente a `bombvault-` mais 14 dígitos. Os instantâneos de segurança, os seus próprios instantâneos e os automáticos nunca são tocados. Para remover um à mão:

```
zfs destroy -r cache/appdata@bombvault-20260924021500
```

## Anomalias {#anomalies}

Um filho que foi esvaziado mal altera o total de uma árvore grande, por isso a deteção de anomalias vigia cada conjunto de dados de um elemento por si: o tamanho, o número de ficheiros, os dados novos e o tempo do restic têm cada um o seu próprio histórico. Esse histórico pertence ao nome do conjunto de dados, por isso mantém-se quando a árvore é mais tarde guardada por outro elemento.

Um conjunto de dados que a execução anterior guardou e que esta não conseguiu ler conta como esvaziado, desde que a seleção do elemento não tenha mudado. Isso abrange uma chave não carregada, um conjunto de dados não montado e um que desapareceu da árvore. Um filho que exclui por si próprio altera a seleção, por isso o seu histórico recomeça do zero. Enquanto uma deteção de dados perdidos estiver aberta, a retenção mantém os backups antigos apenas desse conjunto de dados e limpa o resto da árvore como de costume.

No separador **Elementos** da página **Anomalias**, cada conjunto de dados tem uma linha própria sob o seu elemento, e a árvore do elemento nesta página mostra as deteções abertas junto a cada conjunto de dados. A ligação numa deteção abre o painel de restauro do elemento no último backup bom do conjunto de dados. Se uma execução chega ao fim é avaliado para o elemento inteiro, porque uma execução tem êxito ou falha como um todo.

As verificações em si estão descritas em [Funcionalidades](features.md). Um assistente ligado através do [servidor MCP](mcp.md) pode listar os pontos de restauro de um elemento ZFS, iniciar o seu backup e ler as deteções, mas confirmar uma deteção faz-se na página **Anomalias**.

## Códigos de motivo {#reason-codes}

A página, o histórico de execuções e as notificações indicam um problema com um destes códigos. A maioria tem também a solução ao lado, na página.

| Código | Significado | O que fazer |
|---|---|---|
| `ssh-missing` | A ligação SSH não está configurada neste contentor. | Configure a ligação SSH como para os backups de VM. |
| `host-placeholder` | Host SSH: Address continua a ser o valor de exemplo, e `host.docker.internal` também não respondeu. | Defina Host SSH: Address como o IP LAN deste servidor. |
| `host-fallback` | Host SSH: Address continua a ser o valor de exemplo, e `host.docker.internal` funciona. | Nada, ou defina o IP LAN. |
| `ssh-unreachable` | Não é possível chegar ao servidor por SSH. | Verifique o endereço e a porta, e que o SSH está ligado. |
| `ssh-auth` | O servidor recusou a chave do BombVault. | Execute uma vez no servidor o comando mostrado no cartão da ligação. |
| `zfs-not-found` | O anfitrião SSH não tem o comando `zfs`. | Aponte Host SSH: Address para a máquina a que pertencem os pools. |
| `zfs-permission` | O utilizador SSH não pode executar este comando zfs. | Use root, ou veja [TrueNAS SCALE](#truenas). |
| `uri-mismatch` | `LIBVIRT_URI` indica outro anfitrião ou utilizador que os campos SSH. | Ponha-os de acordo, ou esvazie os campos SSH para que ambos venham do URI. |
| `zfs-error` | O zfs reportou outro erro. | Os detalhes mostram a sua mensagem. |
| `propagation-missing` | As novas montagens no anfitrião não chegam ao contentor. | Defina o Access Mode de Host Data como Read/Write - Slave e reinicie o BombVault. |
| `invalid-name` | Um nome de conjunto de dados que o BombVault não aceita. | Mude o nome do conjunto de dados. |
| `name-too-long` | Um conjunto de dados da árvore é demasiado longo para um nome de instantâneo. | Mude-lhe o nome, ou adicione como elemento um conjunto de dados abaixo dele. |
| `invalid-exclude` | Um padrão de exclusão ou um filho excluído não encaixa no elemento. | Corrija a entrada indicada na mensagem. Para deixar de fora um conjunto de dados filho inteiro, desative-o em vez de escrever um padrão. |
| `not-found` | O conjunto de dados não existe no servidor. | Remova o elemento ou volte a criar o conjunto de dados. Os seus backups continuam restauráveis. |
| `not-filesystem` | Isto é um volume, não um sistema de ficheiros. | Veja [Volumes](#volumes). |
| `overlaps-item` | O conjunto de dados sobrepõe-se a um elemento existente. | Veja [Os elementos nunca se sobrepõem](#overlap). |
| `docker-storage` | A árvore contém o armazenamento de imagens do Docker. | Veja [O armazenamento do Docker](#docker-storage). |
| `nothing-readable` | Neste momento nenhum conjunto de dados do elemento pode ser lido. | Veja os códigos dos conjuntos de dados ignorados. |
| `snapshot-failed` | Não foi possível criar o instantâneo. | Os detalhes mostram a mensagem do zfs. |
| `containers-busy` | Um backup de contentor ainda estava a correr quando os contentores tinham de parar. | Volte a tentar mais tarde. As execuções agendadas esperam sozinhas. |
| `consistency-stop-failed` | Não foi possível parar um contentor, por isso não foi tirado nenhum instantâneo. | Verifique o contentor, ou retire-o da lista. |
| `pre-snapshot-failed` | O comando antes do instantâneo falhou. | Os detalhes da execução mostram a sua saída. |
| `container-unknown` | Um contentor da lista não existe. | Retire-o da lista. |
| `container-is-self` | O BombVault não pode parar o próprio contentor. | Retire-o da lista. |
| `leftover-snapshots` | Continuam no servidor instantâneos que o BombVault não conseguiu remover. | Carregue em **Remover agora**, veja [Instantâneos que sobraram](#leftover-snapshots). |
| `zvol` | Um volume na árvore, ignorado. | Veja [Volumes](#volumes). |
| `canmount-off` | Nunca montado (`canmount=off`), ignorado. | Se tiver dados, monte-o ou mova os dados para um conjunto de dados filho. |
| `legacy-mount` | Ponto de montagem legacy, ignorado. | Dê-lhe um ponto de montagem abaixo de `/mnt`. |
| `no-mountpoint` | Sem ponto de montagem, ignorado. | Dê-lhe um ponto de montagem abaixo de `/mnt`. |
| `not-mounted` | Não montado no servidor, ignorado. | Monte-o com `zfs mount`, ou defina `canmount=on`. |
| `key-not-loaded` | Encriptado e a chave não está carregada, ignorado. | `zfs load-key` e depois monte-o. |
| `snapdir-disabled` | O acesso a instantâneos está desligado, ignorado. | `zfs set snapdir=hidden <dataset>`. A pasta `.zfs` continua oculta. |
| `not-visible` | O BombVault não vê o ponto de montagem do conjunto de dados. | Mova o ponto de montagem para baixo do caminho de Host Data, ou mapeie-o no contentor no mesmo caminho com Read/Write - Slave. |
| `shfs-only` | O conjunto de dados só é visível através de `/mnt/user`, que esconde os instantâneos. | Mapeie `/mnt`, e não `/mnt/user`, como Host Data. |
| `snapshot-not-visible` | O instantâneo foi criado mas não apareceu dentro do BombVault. | Execute **Testar o acesso aos instantâneos**; veja abaixo. |
| `snapshot-loop` | O instantâneo não chegou ao BombVault porque Host Data não deixa passar montagens novas. | Defina o Access Mode de Host Data como Read/Write - Slave e reinicie o BombVault. |
| `backup-failed` | O restic falhou para este conjunto de dados. | Os detalhes da execução mostram porquê. |
| `not-reached` | A execução terminou antes deste conjunto de dados. | Volte a executar o backup. |
| `gone` | O conjunto de dados já não está no servidor. | Nada. Os seus backups continuam restauráveis. |
| `read-only-mount` | O BombVault só consegue ler o conjunto de dados, por isso não pode restaurar para dentro dele. | Defina o mapeamento como Read/Write - Slave, ou restaure para uma pasta. |
| `destination-not-mounted` | A pasta não está num pool ou partilha montados. | Escolha uma pasta num pool ou partilha. |
| `not-enough-space` | Não há espaço livre suficiente no destino. | Liberte espaço ou escolha outra pasta. |
| `safety-snapshot-failed` | Não foi possível tirar o instantâneo de segurança, por isso nada foi restaurado. | Os detalhes mostram a mensagem do zfs. |
| `safety-name-too-long` | O nome do conjunto de dados é demasiado longo para um instantâneo de segurança. | Desligue o instantâneo de segurança, ou restaure para uma pasta. |

### Verificar o que o contentor vê {#mountinfo}

**Testar o acesso aos instantâneos** num elemento tira um instantâneo real da sua árvore, procura-o dentro do BombVault para cada conjunto de dados e volta a removê-lo. É a forma mais rápida de comprovar todo o caminho antes da primeira execução agendada.

Para ver por si, execute isto no servidor:

```
docker exec BombVault grep zfs /proc/self/mountinfo
```

Cada linha é uma montagem dentro do contentor. A linha de um conjunto de dados mostra o seu caminho dentro do contentor (abaixo de `/host/user`) e o nome do conjunto de dados. Um campo `master:N` nessa linha significa que a montagem recebe as montagens que o anfitrião faz mais tarde, que é o que o acesso aos instantâneos precisa. Se faltar, defina o Access Mode de Host Data como Read/Write - Slave e reinicie o BombVault.

## TrueNAS SCALE {#truenas}

- Quando `LIBVIRT_URI` está definido (como para os backups de VM no TrueNAS), o BombVault vai buscar ao URI o anfitrião, o utilizador e a porta SSH dos seus comandos zfs, cada um que não esteja definido à parte. Sem backups de VM, defina antes `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` e `LIBVIRT_SSH_PORT`. Adicione as variáveis em **Additional Environment Variables**.
- Um utilizador que não seja root precisa de permissão no conjunto de dados de topo do elemento, que cobre então todos os conjuntos de dados abaixo dele:

  ```
  zfs allow <user> snapshot,destroy,mount <dataset>
  ```

  Uma sessão SSH sem root no TrueNAS não tem `/usr/sbin` no caminho; o BombVault chama então `/usr/sbin/zfs` diretamente.
- O **Host Data** da app tem de ser um caminho do anfitrião acima dos conjuntos de dados, por exemplo `/mnt/tank`, e não um ixVolume. Com um caminho do anfitrião, a app passa ao BombVault as montagens novas do anfitrião (`rslave`), que é o que o acesso aos instantâneos precisa.
