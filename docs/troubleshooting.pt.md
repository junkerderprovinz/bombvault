# Resolução de problemas

Um FAQ curto. Para a tabela completa de resolução de problemas do lado do host do backup de VMs por SSH (permissão negada, verificação da chave do host, variáveis de template em falta e mais), consulta o [guia de backup de VMs por SSH](vm-backup-ssh-setup.md).

## Alguma coisa não está corretamente ligada {#something-is-not-wired-up-correctly}

Adiciona o cartão **Integração com o host** à visão geral em **Personalizar**. A verificação de integração com o host sonda cada montagem e CLI (socket Docker, libvirt, restic, qemu-img, rclone) e comunica quaisquer peças em falta, e **Verificar agora** executa-a de novo. Começa por aqui antes de assumir um bug: uma montagem em falta ou um host inacessível aparece de imediato.

## Não consigo alcançar a interface web {#i-cannot-reach-the-web-ui}

O BombVault serve HTTPS logo de início na porta `3443` (certificado autoassinado), por isso abre `https://<ip-do-teu-unraid>:3443`. Aceita o aviso do certificado autoassinado, ou coloca o BombVault por trás de um proxy reverso com o teu próprio certificado. Se correres com `HTTP_ONLY=true`, serve HTTP simples na porta `3000` em vez disso (destinado a uso por trás de um proxy que termina o TLS).

## Perdi a minha APP_KEY {#i-lost-my-app_key}

A `APP_KEY` deriva a palavra-passe do repositório restic. Sem ela, sem o kit de recuperação e sem um depósito da chave no teu grupo, os backups encriptados não podem ser recuperados. É por isso que a visão geral mostra **Guarda o teu kit de recuperação** até o descarregares. Vê [Externo e recuperação](offsite-recovery.md#recovery-kit). Gera uma chave com `openssl rand -hex 32` e guarda-a fora do servidor antes de depender de qualquer backup.

## Esqueci-me da palavra-passe de acesso {#i-forgot-the-login-password}

Escolhe **Esqueceste-te da palavra-passe?** na página de início de sessão. O BombVault escreve um código de 12 caracteres no registo do seu container (no Unraid: separador Docker, ícone do BombVault, **Logs**). Escreve-o com uma palavra-passe nova no prazo de 10 minutos. A reposição também desliga a autenticação de dois fatores, remove todas as chaves de acesso, revoga todos os tokens de API, chaves MCP e inícios de sessão OAuth, termina todas as outras sessões e desliga o emparelhamento de telemóveis. Volta a adicionar as tuas chaves de acesso em **Definições, Segurança**, liga aí de novo o emparelhamento de telemóveis e cria tokens e chaves novos onde ainda precisares deles. Vê [Modelo de segurança](configuration.md#login).

## Os backups pararam e dizem que a chave já não pode ser aberta {#backups-stopped-and-say-the-key-can-no-longer-be-opened}

Este servidor assumiu a chave de backup de outro servidor no assistente de emergência, e a `APP_KEY` foi alterada depois. A chave assumida está selada com a APP_KEY antiga, por isso o BombVault não a consegue abrir e prefere não fazer backups a escrever com a chave errada. **Precisa de atenção** na visão geral indica-o. Repõe a APP_KEY no que era, ou carrega o kit de recuperação outra vez na página Restauração em **Emergência**.

## Um telemóvel vai parar à página de início de sessão {#a-phone-lands-on-the-login-page}

Um telemóvel emparelhado só recebe a sessão através do grupo enquanto **Os telemóveis podem emparelhar com as doze palavras do grupo** estiver ligado em **Definições, Segurança**. Uma reposição da palavra-passe desliga-o. Uma aplicação mais antiga do que o servidor também não recebe sessão até ser atualizada. Um servidor adicionado pelo endereço pede sempre a palavra-passe. Um telemóvel removido da página Instâncias regressa desligado quando a sua aplicação volta a abrir. Liga **Consultar esta instância** em **Monitorização** nos seus detalhes para o deixar entrar.

## O backup de VMs não se liga {#vm-backup-will-not-connect}

O backup de VMs comunica com o libvirt por SSH, nunca por uma montagem.

- Confirma que o SSH está ativado no host e que a chave pública do BombVault está autorizada em `/root/.ssh/authorized_keys` (Definições, Integrações, SSH do anfitrião mostra a chave e um botão **Testar ligação**).
- Numa rede `br0.x` personalizada, define `LIBVIRT_HOST` com o IP LAN do teu Unraid (o container não consegue alcançar o host via `host.docker.internal` aí). Ativa **Settings, Docker, Host access to custom networks**.
- Se alteraste a porta SSH do Unraid, define `LIBVIRT_SSH_PORT` para corresponder.
- No TrueNAS, define `LIBVIRT_URI` com o socket libvirt próprio do TrueNAS, dá à app a capability `SETPCAP` e adiciona a chave ao root em **Credentials, Users** (vê [TrueNAS SCALE](getting-started.md#truenas)).
- O diagnóstico completo passo a passo (teste de alcance, encaminhamento de VLAN, `Permission denied (publickey)`, `Host key verification failed`) está no [guia de backup de VMs por SSH](vm-backup-ssh-setup.md).

## Um instantâneo a quente de VM não correu {#a-live-vm-snapshot-did-not-run}

Os instantâneos a quente precisam do disco em `/mnt/cache` (ou `/mnt/diskX`), não em `/mnt/user`. Com o agente convidado qemu na VM, o instantâneo fica em repouso, e sem ele o instantâneo é consistente em caso de falha. Numa VM desligada, o a quente recorre automaticamente ao ordenado. Um backup ordenado desliga a VM, faz backup dos discos e depois reinicia-a, por isso é sempre consistente. Espera até dez minutos que o convidado se desligue e nunca o força a desligar, para que um convidado Windows a instalar atualizações não seja cortado a meio. Um convidado que ainda esteja a correr depois disso fica em paz, e esse backup falha com o motivo.

## Um backup falhou com "repository is already locked" {#a-backup-failed-with-repository-is-already-locked}

Isto é geralmente um bloqueio restic órfão deixado para trás quando o container foi atualizado ou reiniciado a meio de uma operação. O BombVault deteta um bloqueio comprovadamente órfão, limpa-o e tenta de novo uma vez, automaticamente. Depois de um reinício, espera até esse bloqueio passar dez minutos sem ser renovado. Um restic ainda em execução renova o bloqueio a cada cinco minutos, e o BombVault nunca remove um bloqueio que ainda está a ser renovado, porque pode pertencer a um segundo BombVault que escreve no mesmo repositório. Um bloqueio que passou 30 minutos sem ser renovado é removido, seja quem for que o criou. Se o erro persistir, abre o cartão **Verificar os backups** em **Definições, Verificação** e escolhe **Desbloquear** em **Mais** na linha da secção afetada para limpar um bloqueio preso à mão. **Desbloquear** também deixa ficar um bloqueio que ainda está a ser renovado e diz-te quando foi renovado pela última vez. Um problema genuíno continua a vir ao de cima em vez de ser escondido.

## A minha cópia externa não aconteceu após um backup {#my-off-site-copy-did-not-happen-after-a-backup}

A replicação externa é de melhor esforço por conceção, por isso um percalço externo nunca faz o backup local falhar. Verifica quando o destino copia: o cartão **Copiar** diz **Depois de cada backup** ou indica um plano próprio, e **Definições, Agendamento, Cópias externas por secção** pode dar a uma secção um plano próprio. Usa **Replicar agora** na linha da secção do cartão **Cópias externas** em **Definições, Destinos** para uma execução a pedido, e observa o cabeçalho de estado da visão geral, que diz **Cópia externa em curso** enquanto corre. Uma cópia falhada aparece em **Precisa de atenção**.

## O certificado de um rest-server não é de confiança {#a-rest-servers-certificate-is-not-trusted}

O restic recusa um certificado que não conhece, e o erro diz onde o colocar. Copia o certificado autoassinado do rest-server, ou o da autoridade que o assinou, como ficheiro PEM para a pasta `certs` da pasta de dados do BombVault (`/config/certs` no container, `appdata/bombvault/certs` no Unraid) e testa de novo. Não é preciso reiniciar.

## Um backup diz que os dados fora de Host Data não foram copiados {#data-outside-host-data}

O container guarda dados numa pasta que o BombVault não consegue ler através do Host Data. O backup guarda a definição e todas as pastas a que chega, pinta de laranja a sua linha no registo de atividade e indica as pastas que deixou de fora. Há dois casos comuns: um ixVolume no TrueNAS em `/mnt/.ix-apps` e um volume do Docker com nome em `/var/lib/docker/volumes`, como o volume da base de dados de uma stack compose. O cartão **Pastas a copiar** do separador **Definições** do container mostra cada uma dessas pastas com o caminho dentro do container e o caminho no host, marcada a vermelho.

Um volume anónimo do Docker não é comunicado. O Docker cria-o sozinho para uma pasta que a imagem declara e que ninguém mapeou, dá-lhe um nome de 64 dígitos hexadecimais e guarda-o em `/var/lib/docker/volumes`. Normalmente contém uma cache, por isso o backup fica verde e o cartão apenas o mostra, com uma nota de que é anónimo.

- **Queres copiar esses dados.** Mapeia a pasta para um caminho do host abaixo do Host Data, por exemplo uma pasta da tua partilha appdata, e o próximo backup inclui-a.
- **Não precisas deles.** Adiciona um dos seus dois caminhos como uma linha no cartão **Padrões de exclusão** do mesmo separador: o caminho dentro do container, como `/cache`, ou o caminho no host que o aviso indica, como `/var/lib/docker/volumes/jellyfin_cache/_data`. A partir do próximo backup, o aviso deixa essa pasta de fora, e a pré-visualização por baixo do campo diz porquê. Uma linha para uma pasta acima também serve, um nome simples ou `/` não.

## Um backup aparece como Adiado {#deferred-backups}

A app do container estava ocupada, por isso o seu backup espera e volta a ser tentado sozinho. A linha no registo de atividade diz porquê e quando é a próxima tentativa. O cartão **Quando a app está ocupada** no separador **Definições** do container diz o mesmo e quando termina a espera. Não é preciso fazer nada. Para fazer o backup já, usa **Fazer backup agora**: um backup que inicias tu nunca espera, nem por um código de saída 75.

- **A tua própria verificação.** Faz o **Comando pré-backup** terminar com o código 75 enquanto a app está ocupada, por exemplo `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Qualquer outro código diferente de 0 faz falhar o backup como antes, por isso um script avariado continua a aparecer em **Precisa de atenção**.
- **O Plex, o Jellyfin ou o Emby nunca é consultado.** O BombVault pergunta primeiro ao servidor no endereço do próprio container e, quando aí não há resposta, corre curl ou wget dentro do container. Uma imagem sem nenhum dos dois não pode ser consultada através de si mesma. O registo indica-o então. Com **Fazer backup quando o servidor não responde** ligado, um backup assim corre na mesma, com a opção desligada espera.
- **O Jellyfin ou o Emby recusa.** Verifica a **Chave de API**. Cria-se no painel do servidor, em API keys.
- **O Plex recusa.** O BombVault lê o token do `Preferences.xml` na pasta montada em `/config`. Um servidor onde nunca se iniciou sessão não tem token e só responde onde permite acesso sem ele.
- **Um filme em pausa deixa o backup à espera a noite toda.** Define **Ignorar sessões em pausa após (min)**, por exemplo para 30.
- **A verificação de tráfego nunca deixa passar o backup.** Sobe **Ocupado a partir de (KiB/s)** acima do que o container envia quando ninguém o usa. Logo depois de o BombVault arrancar, a verificação espera até ter observado o container durante **Calmo durante (min)** minutos.
- **O backup nunca corre à noite.** Com uma janela definida, as tentativas só acontecem dentro dela. Uma espera que chega ao prazo fora da janela termina na mesma no prazo.

## Um backup grande foi terminado por falta de memória {#a-large-backup-was-killed-for-running-out-of-memory}

Define **Limite de memória por backup (MiB, 0 = nenhum)** no cartão **Desempenho** em **Definições, Sistema**, bem abaixo do que o container pode usar. Perto do limite, o restic liberta memória mais vezes, o que custa alguma velocidade, por isso o backup abranda em vez de ser terminado.

## Uma restauração abortou antes de começar {#a-restore-aborted-before-it-started}

Antes de qualquer coisa ser parada ou removida, a restauração corre uma verificação de conflitos pré-voo: verifica que o IP estático do container e as portas do host publicadas estão livres. Se outro container já detém uma, aborta com uma mensagem clara e acionável em vez de deixar uma restauração a meio. Liberta a porta ou o IP em conflito e tenta de novo.

Se a rede do container não existir, por exemplo porque a sua app do TrueNAS foi apagada, a verificação antes da restauração indica a rede e a restauração recusa-se antes de alterar o que quer que seja. Reinstala a app no TrueNAS com o mesmo nome, o que volta a criar a rede, e depois restaura os seus dados.

## Uma exportação simples falhou em vez de escrever um ficheiro {#a-plain-export-failed-instead-of-writing-a-file}

Se **Cifrar exportações com age** estiver ligado em **Definições, Sistema** mas não estiver definido nenhum destinatário válido, uma exportação falha com um erro claro em vez de escrever texto simples. Adiciona um destinatário válido (uma chave pública age ou uma chave pública SSH), ou desliga a cifragem se pretenderes que a exportação seja texto simples. Vê [Exportações simples](backups.md#exports).

## Um despejo da base de dados falhou {#a-database-dump-failed}

Um despejo falhado nunca faz falhar o backup à volta dele. Fica registado como uma execução falhada própria, e o motivo diz o que corrigir.

- **Início de sessão recusado.** O despejo entra com as variáveis de palavra-passe do próprio container (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` ou as versões `_FILE`). Confirma-as no container da base de dados. Uma variável `_FILE` que aponta para um segredo que o utilizador do container não consegue ler falha da mesma maneira.
- **Privilégios em falta.** Com uma palavra-passe de root aleatória, o despejo só consegue entrar como utilizador da aplicação, pelo que contém apenas aquela base, e o MySQL 8.4 e mais recentes podem recusá-lo de todo. Dá ao container uma palavra-passe de root a sério, ou desliga-lhe o despejo.
- **As tabelas de sistema precisam de atualização.** O MariaDB recusa o despejo quando as suas tabelas de sistema vêm de uma versão mais antiga (erro 1558). Acrescenta a variável `MARIADB_AUTO_UPGRADE=1` e reinicia o container, ou corre `mariadb-upgrade` lá dentro uma vez.
- **Sem ferramenta de despejo.** Uma imagem enxuta ou feita à mão sem `pg_dump`, `mysqldump` ou `mariadb-dump` não pode ser despejada. Usa a imagem oficial, ou desliga o despejo.
- **Um limite de tempo.** Um despejo tem `DB_DUMP_MAX_HOURS` (6 por predefinição), o backup à volta tem `BACKUP_MAX_HOURS`, e um despejo que deixa de avançar é cortado ao fim de `BACKUP_STALL_HOURS`. A causa habitual do último é um bloqueio que a aplicação mantém. Sobe o limite que disparou, ou faz o despejo enquanto a aplicação está sossegada.
- **O container está em pausa ou a reiniciar.** O despejo fala com o servidor em funcionamento. Se o container reinicia sem parar, o seu próprio registo diz porquê.
- **Um despejo danificado não pôde ser removido.** Um despejo que o BombVault não conseguiu terminar é apagado. Quando esse apagamento falha, o despejo fica na lista marcado como danificado e podes eliminá-lo aí.

## Uma importação falhou {#an-import-failed}

Uma importação pára o container, põe a pasta de dados de lado e deixa a imagem criar uma vazia no lugar. Se falhar um passo antes da importação em si, a pasta antiga volta sozinha ao sítio. Se falhar a importação, o container fica com a pasta nova e a antiga permanece ao lado como `<pasta de dados>.bombvault-before-import-<data e hora>`. A mensagem de erro da execução indica o caminho exato.

Uma pasta de dados que é um conjunto de dados próprio, como no TrueNAS, não pode mudar de nome. A importação deixa então a pasta no lugar e move o seu conteúdo para a pasta guardada ao lado, por isso o conjunto de dados acima precisa de espaço para ele. A importação verifica isso antes de parar o container.

Para a repor à mão: pára o container, muda o nome da pasta de dados atual para a tirar do caminho, muda o nome da pasta guardada de volta ao original e arranca o container. Num conjunto de dados, move o conteúdo em vez de mudar o nome das pastas. No Unraid, o gestor de ficheiros no separador Shares faz isto.

## Uma cópia de um conjunto de dados ZFS falhou ou ignorou um conjunto {#zfs-datasets}

Cada problema traz um código de motivo entre parênteses retos, e a página [Conjuntos de dados ZFS](zfs-datasets.md#reason-codes) lista-os todos com a solução. Os três mais comuns:

- **`snapshot-loop`**: o instantâneo não chegou ao BombVault porque o Host Data não passa as novas montagens. Edita o container, põe o Access Mode do Host Data em Read/Write - Slave e reinicia o BombVault.
- **`key-not-loaded`**: um conjunto de dados cifrado cuja chave não está carregada é ignorado. Carrega a chave com `zfs load-key` e monta o conjunto de dados. O próximo backup inclui-o.
- **`ssh-auth`**: o servidor recusou a chave do BombVault. **Ligação ZFS a este servidor**, que se abre com **Ver ligação** na página Backups, mostra o comando que a autoriza. Executa-o uma vez no servidor.

## Uma entrada fica em "A aprender N/10" {#an-entry-stays-at-learning-n10}

A maioria das verificações de anomalias começa após 10 backups bem-sucedidos de uma entrada, e a contagem recomeça após **Marcar como esperada** e depois de a seleção da entrada mudar. Uma entrada sem agendamento não aprende, e um container sem appdata não tem com que aprender, como indica o seu distintivo. O **Progresso da aprendizagem** na página Anomalias mostra um anel de dez partes por entrada.

## A retenção deixou de apagar os backups antigos de uma entrada {#retention-stopped-deleting-old-backups-of-one-entry}

Uma anomalia crítica em aberto está a segurá-los: a origem da entrada está quase vazia, encolheu muito, ou um backup voltou a guardar a maior parte dos dados. Abre a anomalia a partir do distintivo da entrada. Se faltarem dados ou tiverem sido cifrados, restaura primeiro a partir do último backup bom indicado. Depois confirma a anomalia, ou marca-a como esperada se a alteração foi tua, e a execução seguinte poda como de costume. A pré-visualização da retenção assinala essa entrada como mantida. Numa entrada ZFS só o conjunto de dados indicado na anomalia mantém os backups antigos, e os outros conjuntos da árvore são podados como de costume.

## A poda manual diz que algumas entradas foram mantidas {#manual-prune-says-some-entries-were-kept}

A mesma causa: a poda deixa em paz os backups antigos de uma entrada com uma anomalia destas e indica a entrada na sua mensagem. Tudo o resto é podado como de costume.

## Um backup falha com "on no mounted pool or share" {#no-mounted-pool}

No Unraid, o `/mnt` vive na memória, por isso uma pasta lá sem nenhum pool montado por cima desaparece no reinício seguinte e fica escondida quando o pool volta. O BombVault recusa escrever lá um backup ou um ZIP do flash, também quando já está nessa pasta um repositório de uma escrita anterior. Monta o pool e corre o backup outra vez.

## A importação do histórico diz que um repositório não pôde ser lido {#history-import-says-a-repository-could-not-be-read}

Depois da atualização, o BombVault lê uma vez o tamanho dos backups anteriores de cada repositório. Um repositório que não estava acessível nesse momento, como um destino externo em baixo ou uma partilha não montada, é contado no cartão **Anomalias** em **Definições, Verificação** e tentado de novo uma vez por dia. Entretanto, as suas entradas aprendem com os backups novos.

## O aviso de espaço em disco não coincide com o painel do Unraid {#the-disk-space-warning-does-not-match-the-unraid-dashboard}

Na partilha de utilizador do Unraid (`/mnt/user`) o espaço livre é o de todo o array, não o de um disco. Os destinos remotos que o BombVault alcança através do rclone, como SFTP, a Hetzner Storage Box, SMB, WebDAV ou um disco na nuvem, são medidos quando o outro lado indica o seu espaço livre, e o mesmo vale para um endereço `sftp:` escrito à mão. Os repositórios S3, B2 e REST não têm valor: o seu destino diz **Tamanho desconhecido**, e o cartão **Anomalias** lista-os como não medidos.

## Um assistente de IA não consegue ligar-se {#an-ai-assistant-cannot-connect}

A [página do servidor MCP](mcp.md#troubleshooting) lista o que significa cada código de estado e cada recusa do ponto de ligação MCP e o que fazer.

## O container continua a reiniciar ou parece não saudável {#the-container-keeps-restarting-or-looks-unhealthy}

O BombVault comunica saudável ou não saudável a partir do seu próprio `/api/health`. Uma ferramenta de autorrecuperação (como o Autoheal) pode reiniciá-lo automaticamente se o motor alguma vez encravar. Verifica o registo do container e o cartão **Integração com o host** para a causa subjacente.

## Um destino recusa-se a guardar {#location-refused}

O BombVault verifica um destino antes de o guardar, e a recusa diz porquê numa frase. As mais habituais:

- **"This location lies inside another repository or target, or contains one."** Um repositório dentro de um repositório estragaria os dois. A mensagem nomeia o outro lugar. Escolhe outra pasta, ou remove primeiro essa.
- **É um repositório que configuraste.** O destino é uma das linhas em **Repositórios**, ou está dentro ou à volta de uma. Escolhe outro lugar, ou remove primeiro esse repositório.
- **O mesmo lugar que a pasta própria de uma secção.** Uma cópia para a pasta própria de uma secção copiaria os seus backups para cima deles mesmos em cada execução, por isso é recusada. Escolhe outro lugar, ou vê [A pasta própria de uma secção](offsite-recovery.md#own-folder).
- **"That is not an off-site target of this domain"** vinha de uma ligação a um destino que foi removido. Uma versão atual limpa estas ligações no arranque e numa importação. Se ainda a vires, atualiza o BombVault.
- **Um caminho completo do host** como `/mnt/user/backups`. O BombVault corre num container, por isso quer o caminho abaixo do Host Data, `user/backups`, e a mensagem nomeia o que deves escrever.

## Um destino remoto diz que o seu repositório desapareceu {#repository-gone}

Um destino que guardava backups e de repente não tem repositório não recebe sozinho um novo, vazio, porque esse esconderia o verdadeiro quando um disco voltasse. Um rest-server cujo disco não montou é a causa habitual, por isso monta o disco outra vez e volta a correr o backup ou a cópia. Só se esvaziaste ou eliminaste o repositório antigo de propósito, carrega em **Criar aqui um repositório novo** na página do destino. Recomeça para todas as secções desse destino de uma vez.

## O início de sessão na nuvem expirou {#cloud-sign-in-expired}

O teste ou a cópia seguinte diz então que o início de sessão expirou. **Alterar a ligação** na página do destino inicia sessão de novo.

- **Box, Jottacloud e outros com tokens de renovação de uso único:** o BombVault guarda o token que o rclone renovou, também entre reinícios e no kit de recuperação. Depois de uma restauração a partir de um backup de definições mais antigo, o token que ele traz pode ter expirado, por isso inicia sessão mais uma vez. O repositório fica como está.
- **iCloud Drive:** a Apple termina o início de sessão ao fim de 30 dias. Precisa de um novo token de confiança e cookies do `rclone config`.
- **OneDrive:** um início de sessão com várias unidades pergunta qual usar, por isso um destino deixado sem escolha fica inutilizável até escolheres. Para o OneDrive Business e o Google Drive, vê a nota sobre dados podados em [Destinos](offsite-recovery.md#destinations).

## "The off-site copy of the last backup is still running" {#copy-still-running}

O backup anterior terminou, e a sua cópia ainda está a caminho, o que bloqueia um novo backup dessa secção. Espera até o cabeçalho de estado deixar de dizer **Cópia externa em curso**. Os botões que iniciam um backup dizem o mesmo enquanto esperam. A primeira cópia para um destino novo pode demorar horas, vê o [exemplo S3](offsite-recovery.md#example-s3).

## Uma réplica ZFS pára porque o destino tem instantâneos mais recentes {#replica-target-newer}

O destino tem instantâneos mais recentes do que o mais recente que partilha com este servidor, por exemplo porque o conjunto de dados aqui foi revertido ou a cópia no destino foi alterada. O cartão Réplica e o histórico de execuções nomeiam o instantâneo em **Reverter o destino para**. Corre `zfs rollback -r` para esse instantâneo no destino, para cada conjunto de dados que a execução nomear, e carrega em **Replicar agora**. Vê [Quanto tempo ficam os instantâneos](zfs-datasets.md#replica-retention).

## Uma VM com instantâneos do Unraid falha e nomeia um ficheiro {#vm-chain}

O BombVault segue a cadeia de backing de cada disco e faz backup de todos os ficheiros dela. Uma cadeia que não se consegue seguir até ao fim faz o backup falhar em vez de deixar de fora parte do disco, e a mensagem nomeia o ficheiro. Verifica que o ficheiro existe abaixo de `/mnt` e que o instantâneo ainda está listado no Gestor de VMs. Um ficheiro fora do Host Data não pode ser lido de todo. Vê [VMs com instantâneos do Unraid](vm-backup-ssh-setup.md#unraid-snapshots).

## As cópias externas aguardam a tua decisão depois de uma reconstrução {#copies-paused}

Depois de **Descobrir backups** ou do assistente de emergência sem definições restauradas, ou quando uma secção encontra backups seus de antes desta base de dados, as cópias externas dessa secção esperam, porque o BombVault já não sabe que entradas deixaste de fora. **Precisa de atenção** e a página do destino perguntam por isso. Carrega em **Assumir como backups deste servidor** para continuar, ou em **Verificar primeiro as regras** para veres as regras da secção antes disso. Importar um ficheiro de definições não toma a decisão por ti. Vê [Depois de uma reconstrução](offsite-recovery.md#after-a-rebuild).

## As notificações não chegam {#notifications-do-not-arrive}

Começa por **Enviar teste** no canal, em **Definições, Notificações**. A linha por baixo do canal diz o que aconteceu.

- **Nada é enviado.** Verifica se o canal está ligado e vê o seu **Notificar**. Com **Nunca** não envia nada, e com **Apenas em caso de falha** uma execução boa fica em silêncio. Com **Resumir execuções agendadas** ligado, uma execução agendada envia um resumo no fim.
- **Um endereço novo.** Depois de mudares para onde um canal envia, por exemplo o servidor do Matrix, do Gotify ou do ntfy, ou o host de um canal de e-mail, o campo pede outra vez o token ou a palavra-passe, e o novo endereço só fica guardado quando o escreves. O BombVault não envia o segredo antigo para um sítio novo.
- **O Gotify** diz que o servidor recusou o token da aplicação: copia-o outra vez da aplicação no Gotify. O **URL do servidor** é o endereço base, sem `/message`.
- **Discord, Slack e ntfy** são canais próprios. Um endereço do Discord num canal **Webhook** recebe JSON que o Discord não entende, por isso junta um canal **Discord** para ele.
- **Webhook:** um marcador que o BombVault não conhece fica como está escrito, por isso confirma a ortografia, por exemplo `%%title%%`. Para um corpo JSON mantém o cabeçalho `Content-Type` em `application/json`, para que os valores sejam escapados como JSON.
- As **Notificações do Unraid** correm pela ligação SSH do anfitrião, por isso testa-a primeiro em **Definições, Integrações**. Noutra plataforma ficam desligadas, e o canal di-lo.
- **Um e-mail de notificação nunca chega.** Confirma a **Encriptação** contra a porta, normalmente **STARTTLS** na 587 ou **TLS (implicit)** na 465.

Os pings do Healthchecks não dependem de **Notificar**, vê [Healthchecks](notifications.md#healthchecks).

## A aplicação Android não consegue abrir um servidor {#android-cannot-open}

O ecrã inicial diz porquê no cartão do servidor:

- **Não ligada:** nem o endereço do servidor nem o relay respondem. Em **Sem relay**, a aplicação só alcança um servidor no seu próprio endereço.
- **Verificar certificado:** o servidor mostra outro certificado, diferente daquele em que confiaste. A aplicação só o abre depois de confiares no novo, por isso garante que a mudança é tua.
- **Início de sessão necessário:** um servidor adicionado pelo endereço pede a palavra-passe. Vê também [Um telemóvel vai parar à página de início de sessão](#a-phone-lands-on-the-login-page).
- **A chave mudou:** o ecrã inicial mostra a impressão digital da chave nova, e a página só abre depois de confiares nela. Vê [Fora de casa](android.md#away).

## Outro utilizador não consegue ler o repositório local {#repository-permissions}

O repositório local continua legível para outros utilizadores, como um utilizador de sincronização por SMB. Antes da 10.4.1, os ficheiros que uma poda, etiqueta, recolha, mudança de secção, eliminação ou backup de stack escrevia podiam ficar ilegíveis até à noite seguinte. Atualiza o BombVault e passam a ficar legíveis logo. O repositório do auto-backup continua legível só por root, de propósito, porque guarda as definições do BombVault.

## Ainda preso? {#still-stuck}

- Lê as páginas completas de [Configuração](configuration.md) e [Externo e recuperação](offsite-recovery.md).
- O **Relatório de erro** no cartão **Ajuda** em **Definições, Info** descarrega um ZIP para um issue do GitHub: a verificação do host, a tua configuração com todos os segredos removidos, as execuções recentes, o que está agendado a seguir e o registo recente desde que o container arrancou pela última vez. Traz também o último despejo de cada base de dados, as entradas ZFS com as suas montagens, as anomalias em aberto e quantas chaves MCP existem. As palavras-passe, os tokens e a configuração do rclone são removidos, e o pacote di-lo no seu próprio manifesto. Precisa de uma palavra-passe de acesso. Para uma falha que reiniciou o container, olha para o `docker logs`.
- Pergunta na [thread de suporte do Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Abre uma [issue no GitHub](https://github.com/junkerderprovinz/bombvault/issues).
