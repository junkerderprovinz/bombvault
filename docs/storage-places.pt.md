# Lugares de armazenamento

Um lugar de armazenamento é um sítio onde o BombVault guarda backups: uma pasta neste Unraid, uma partilha num NAS, um bucket num provedor de nuvem, um rest-server, uma conta SFTP ou um Nextcloud. Ligue cada lugar uma vez, em **Definições, Armazenamento**, e as credenciais, a retenção, a proteção e a localização pertencem a esse lugar. Os cinco domínios (containers, VMs, o flash, a configuração do próprio BombVault e os conjuntos de ficheiros) escolhem depois entre os lugares: onde cada domínio é guardado e para onde é copiado.

## Adicionar um lugar {#add-a-place}

**Adicionar lugar** abre uma janela com um mosaico por provedor, em três grupos: armazenamento na nuvem, serviços de alojamento próprio, e NAS e este servidor.

1. Escolha um mosaico e preencha o formulário. O botão do olho mostra um segredo que escreveu.
2. **Testar ligação** verifica o lugar e não cria nada. Para a pasta de cada domínio indica o que encontrou: vazia ou ainda inexistente, já com um repositório restic, ou o erro que impediu a verificação.
3. Dê um nome ao lugar; o nome do provedor já vem preenchido. Para um dispositivo que mantém por conta própria, responda a **Onde está o dispositivo?**. Os provedores de nuvem estão sempre noutro local, e uma pasta neste Unraid está sempre aqui.
4. **Adicionar** guarda o lugar.

Um lugar novo ainda não é usado por nenhum domínio. Escolha-o em **Guardado em** ou **Copiado para** no [cartão Domínios](#domains), ou no cartão de um item, só para esse item.

## Pastas {#folders}

Um lugar tem uma pasta por domínio: `container`, `vms`, `flash`, `config` e `files`, os nomes que as localizações de backup predefinidas usam. As pastas aparecem nos detalhes do lugar e podem ser renomeadas aí (consulte [Mudar um endereço](#addresses)). Um domínio sem pasta num lugar não pode escolher esse lugar.

Quando um domínio usa um lugar nas duas funções, a segunda função recebe um sufixo e a primeira mantém a sua pasta. Um lugar que já recebe as cópias de um domínio guarda os itens enviados diretamente para ele em `<folder>-direct`; um lugar que já guarda um domínio recebe as cópias dele em `<folder>-copies`.

Alguns lugares são eles próprios um repositório restic: um endereço que já continha um repositório quando o lugar foi adicionado, um repositório com nome vindo de uma configuração existente, ou um destino de cópia na raiz de um bucket. Um lugar assim não tem pastas, todos os domínios partilham o seu único repositório, e não aceita uma segunda função. Para guardar mais no mesmo provedor, ligue outro bucket ou outra pasta como lugar próprio.

## Detalhes do lugar {#details}

Cada lugar é uma linha com o seu provedor, aquilo para que é usado e o seu último teste ou cópia. **Testar** verifica todos os endereços do lugar, e **Detalhes** abre as suas definições. Cada alteração nos detalhes fica guardada no momento em que a faz.

- **Geral**: o nome, o interruptor que liga e desliga o lugar, o endereço e, para um dispositivo que mantém por conta própria, **Onde está o dispositivo?** (consulte [Fora das instalações](#off-the-premises)).
- **Retenção**: manter os últimos, diários, semanais e mensais, para cada repositório no lugar. Um lugar novo começa com as regras predefinidas; um lugar com todas as regras a zero nunca apara.
- **Proteção**: o interruptor **Append-only**. O outro lado tem de impor o append-only; com o interruptor ligado, o BombVault nunca poda nem apaga lá. Num rest-server com append-only ligado, **Testar append-only** corre o teste de adulteração contra cada caminho de domínio, cada cópia ligada e cada repositório no lugar e mostra uma única resposta para o lugar inteiro, *eliminações recusadas* ou *eliminações aceites* (consulte [Externo e recuperação](offsite-recovery.md)). Só os lugares remotos têm esta secção, porque nada nesta máquina consegue impedir que um repositório local seja apagado.
- **Acesso**: as credenciais e, no S3, a classe de armazenamento. Um lugar que usa as credenciais partilhadas recebe um conjunto próprio na primeira alteração. Um repositório direto no lugar que as credenciais novas não conseguem abrir mantém as antigas, e a resposta di-lo. Os lugares de pasta, SFTP e rclone não têm esta secção.
- **Limites**: a taxa de envio e de receção e o orçamento de crescimento.
- **Pastas**: um interruptor por domínio, com o nome da sua pasta. Um domínio desligado aqui não pode escolher o lugar.

Reduzir a retenção pede confirmação e diz quantos itens isso afeta; desligar o append-only pede confirmação e diz quantos repositórios no lugar perdem essa proteção. Desligar um lugar desliga todos os repositórios nele; um lugar onde um domínio está guardado não pode ser desligado.

## O cartão Domínios {#domains}

O cartão tem uma linha por domínio, com o seu agendamento, onde está guardado, para onde é copiado e as suas exceções.

- **Guardado em**: enquanto a localização de backup do domínio não tiver backups, o lugar escolhido passa a ser o lugar principal do domínio e a localização muda para lá. Depois de ter backups, a escolha para containers, VMs e conjuntos de ficheiros passa a ser o padrão para itens novos, que a assumem no seu primeiro backup; os itens que já têm backups ficam onde estão, porque o BombVault nunca move um backup. No flash e na configuração do próprio BombVault o lugar principal muda, e os backups já escritos ficam no lugar antigo.
- **Copiado para**: um chip por cada lugar que pode receber as cópias do domínio. Marcar um chip torna o lugar um destino de cópia do domínio; da primeira vez, o BombVault diz de antemão quantos itens e instantâneos e quantos dados a primeira execução envia. Desmarcar interrompe as cópias novas: as cópias que já lá estão ficam e envelhecem segundo a retenção do lugar, e os itens com escolha própria continuam a copiar para lá. Desmarcar o último chip interrompe todas as cópias, também para lugares adicionados mais tarde, até voltar a marcar um. Um lugar desligado aparece como um chip esbatido e não pode ser escolhido.
- **Exceções**: os itens com escolha própria, numa lista com ligações para os seus cartões.
- **Copiar agora** corre de imediato as cópias do domínio.

Um domínio em pausa depois de uma reconstrução através do Descobrir mostra a pausa na sua linha, com **Confirmar padrão** (consulte [Localização por item](offsite-recovery.md#placement)).

## Mudar um endereço {#addresses}

A pasta de um domínio pode ser mudada nos detalhes do lugar, e o mesmo vale para o endereço de um lugar local, por exemplo depois de um repositório ter sido movido à mão para outro disco. O BombVault testa todos os endereços afetados pela alteração e aceita-a quando cada endereço novo está vazio e nada estava guardado no antigo, ou quando cada endereço novo contém o mesmo repositório restic que o antigo. Tudo o resto é recusado, com o número de backups que ainda estão no endereço antigo. Um lugar remoto mantém o seu endereço; para fazer backup para outro sítio, ligue esse sítio como lugar próprio.

O BombVault constrói a lista de lugares a partir da sua própria base de dados e nunca lista um repositório remoto para a preencher; o teste só corre quando altera alguma coisa.

## Remover um lugar {#remove}

Um lugar só pode ser removido enquanto nada o usa: nenhum domínio está guardado lá, nenhum padrão aponta para ele, nenhum item está guardado lá e nenhum repositório direto nele contém itens. Caso contrário, a recusa lista o que o prende. Removê-lo leva consigo os seus destinos de cópia e as suas próprias credenciais, a menos que uma origem de recolha ou outro lugar as use. Nada é apagado no próprio armazenamento, e a confirmação diz quantas cópias ficam lá.

## Sem lugar {#without-a-place}

Um endereço que não encaixa na forma de um lugar mais uma pasta continua a funcionar e aparece em **Sem lugar**, com o seu endereço. Os endereços nativos `b2:`, `gs:` e `swift:` estão entre eles. **Atribuir a um lugar** associa essa linha a um lugar, depois do mesmo teste que ao [mudar um endereço](#addresses). Um destino de cópia sem lugar também aparece na linha do seu domínio, ao lado dos chips, e continua a copiar. Uma linha remota ali tem o seu próprio interruptor **Append-only**, e desligá-lo pede confirmação primeiro, com o número de itens que guardam backups nesse endereço. Um repositório direto segue o interruptor do seu destino.

## Fora das instalações {#off-the-premises}

**Onde está o dispositivo?** tem duas respostas: **Aqui em casa** e **Noutro local**. Uma cópia só conta como um local à parte, para a linha 3-2-1 nos cartões e para as verificações externas do Painel, quando o seu lugar está noutro local. Um segundo disco ou um NAS na mesma casa é uma segunda cópia, não um segundo local. A resposta não muda nenhuma cópia. Os provedores de nuvem estão sempre noutro local e uma pasta neste Unraid está sempre aqui, por isso o formulário não pergunta nesses casos; para qualquer outro lugar, altere a resposta nos seus detalhes. Um lugar noutro local tem a marca **Outro local** na sua linha.

## Tipos de ligação

### Pasta neste Unraid ou num NAS {#kind-local}

O endereço é um caminho sob `/mnt`, escrito sem `/mnt`, por exemplo `user/bombvault`, e a pasta de cada domínio fica por baixo dele: `user/bombvault/container`.

- **Pasta neste Unraid** escolhe entre as partilhas, os discos e as pools.
- **Synology**, **QNAP**, **TrueNAS**, **Outro Unraid** e **Outra partilha** escolhem em `/mnt/remotes`. Monte primeiro a partilha no Unraid, por exemplo com o plugin Unassigned Devices. O Host Data tem de estar montado como Read/Write - Slave, senão uma partilha montada depois de o BombVault arrancar fica invisível até um reinício (consulte [Configuração](configuration.md)).

O seletor de pastas cria uma pasta no sítio onde está com **Nova pasta**. O teste verifica que a pasta está vazia ou não existe e que o BombVault consegue escrever lá.

### S3 {#kind-s3}

O endereço é `s3:https://<endpoint>/<bucket>/<path>`, por exemplo `s3:https://s3.eu-central-003.backblazeb2.com/tower-backups/bombvault`.

- **Backblaze B2** só precisa do ID da chave e da chave de aplicação. O BombVault pergunta ao B2 a que bucket, endpoint S3 e pasta a chave está limitada e constrói o endereço a partir daí. Uma chave com acesso a todos os buckets oferece os seus buckets para escolher.
- **Amazon S3**, **Cloudflare R2**, **Wasabi**, **Hetzner Object Storage**, **Storj**, **IDrive e2**, **Scaleway**, **OVHcloud**, **DigitalOcean Spaces**, **IONOS**, **Contabo**, **Exoscale** e **Vultr** pedem a chave e, quando o provedor precisa, a região, o ID da conta ou o endpoint. O BombVault preenche o endpoint e lista os buckets quando a chave os pode listar; caso contrário, escreva o nome do bucket.
- **Google Cloud Storage** passa pela sua interface S3 com uma chave HMAC, criada nas definições do Cloud Storage em Interoperabilidade. Um ficheiro de conta de serviço não funciona aqui.
- **MinIO**, **SeaweedFS**, **Garage**, **Ceph**, **JuiceFS**, **RustFS**, **Versity S3 Gateway** e **Outro serviço S3** pedem o endereço do serviço e uma chave.

A classe de armazenamento define-se nos detalhes do lugar, limitada aos níveis que um restauro consegue ler sem degelo.

### rest-server {#kind-rest}

O endereço é `rest:<url>/<user>`, por exemplo `rest:https://nas.lan:8000/tower`. O formulário pede o endereço do servidor, um utilizador e uma palavra-passe. Com `--private-repos`, um utilizador só chega a caminhos que começam pelo seu próprio nome, por isso o BombVault coloca o utilizador à frente, a menos que escreva outro caminho. Quando o servidor recusa um caminho fora do caminho do próprio utilizador, o erro di-lo.

O formulário do rest-server traz uma receita pronta a colar para um rest-server em modo append-only com um utilizador para este BombVault. **Mostrar receita** gera uma palavra-passe, mostrada uma vez, e dá uma linha `docker run`, um ficheiro compose e um modelo do Unraid, cada um com a linha `htpasswd` a colocar no servidor; o utilizador e a palavra-passe vão diretamente para o formulário.

**Outro BombVault** lista, acima dos seus próprios campos, as ofertas em aberto que outras instâncias enviaram pela Frota. Aceitar uma adiciona um lugar que guarda cópias apenas do domínio oferecido, porque uma oferta traz um utilizador só para esse domínio. Aceitar na página Frota adiciona o mesmo lugar.

### SFTP {#kind-sftp}

O endereço é `sftp://<user>@<host>:<port>/<path>`, por exemplo `sftp://bv@backup.lan:22/bombvault`. O formulário pede o host, a porta e o utilizador e mostra a chave pública do BombVault. Adicione essa chave ao `~/.ssh/authorized_keys` do utilizador no servidor; não é preciso instalar mais nada lá. O BombVault aceita a chave de host do servidor no primeiro contacto e verifica-a a partir daí.

**Hetzner Storage Box** preenche `<user>.your-storagebox.de` e a porta 23. Instale a chave na box com o comando da própria Hetzner, que pede uma vez a palavra-passe da box:

```sh
echo '<public key>' | ssh -p 23 <user>@<user>.your-storagebox.de install-ssh-key
```

### WebDAV: Nextcloud, ownCloud, OpenCloud {#kind-webdav}

O formulário pede o endereço do servidor, o utilizador e uma palavra-passe de aplicação. Crie a palavra-passe de aplicação nas definições de segurança da conta e indique o ID de utilizador em vez de um endereço de e-mail. O BombVault constrói o caminho WebDAV que o produto usa e passa a ligação ao restic através das variáveis de ambiente do rclone, com a palavra-passe na forma ofuscada do rclone. O endereço fica `rclone:bvp<id>:<path>`, em que `bvp<id>` é um remoto que só existe nesse ambiente; nada é escrito na configuração do rclone.

### Azure Blob {#kind-azure}

O endereço é `azure:<container>:/<path>`. O formulário pede a conta de armazenamento e a sua chave de acesso; depois de **Testar ligação** lista os contentores da conta para escolher, ou pode escrever o nome de um contentor. O BombVault passa a conta e a chave ao restic como `AZURE_ACCOUNT_NAME` e `AZURE_ACCOUNT_KEY`.

### rclone {#kind-rclone}

O endereço é `rclone:<remote>:<path>`. O formulário lista os remotos da configuração rclone do BombVault para escolher. Para substituir essa configuração, cole um `rclone.conf` inteiro em **Configuração rclone** e clique em **Guardar configuração**. Fica guardada de imediato e serve todos os lugares rclone, quer a janela adicione depois um lugar, quer não.

## Cópias entre lugares com credenciais diferentes {#different-credentials}

Um domínio guardado num lugar remoto é a origem das suas cópias. O `restic copy` corre com um único ambiente, e o BombVault junta as credenciais da origem às do destino quando os dois não definem a mesma variável com valores diferentes. Um lugar Nextcloud e um lugar B2 usam variáveis diferentes, por isso um domínio guardado no Nextcloud pode ser copiado para o B2. Duas contas S3 ou dois utilizadores de rest-server precisariam das mesmas variáveis com valores diferentes; o restic não aceita as duas coisas, e o chip no cartão Domínios diz que as credenciais não são compatíveis.
