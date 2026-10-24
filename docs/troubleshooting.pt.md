# Resolução de problemas

Um FAQ curto. Para a tabela completa de resolução de problemas do lado do host do backup de VM por SSH (permissão negada, verificação da chave do host, variáveis de template em falta e mais), consulta o [guia de backup de VM por SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) no GitHub.

## Alguma coisa não está corretamente ligada

Adiciona o cartão **Integração com o host** à visão geral em **Personalizar**. A verificação de integração com o host sonda cada montagem e CLI (socket Docker, libvirt, restic, qemu-img, rclone) e reporta quaisquer peças em falta, e **Verificar agora** executa-a de novo. Começa por aqui antes de assumir um bug: uma montagem em falta ou um host inacessível aparece de imediato.

## Não consigo alcançar a interface web

O BombVault serve HTTPS logo de início na porta `3443` (certificado autoassinado), por isso abre `https://<your-unraid-ip>:3443`. Aceita o aviso do certificado autoassinado, ou coloca o BombVault por trás de um proxy reverso com o teu próprio certificado. Se correres com `HTTP_ONLY=true`, serve HTTP simples na porta `3000` em vez disso (destinado a uso por trás de um proxy que termina o TLS).

## Perdi a minha APP_KEY

A `APP_KEY` deriva a palavra-passe do repositório restic. Sem ela, sem o kit de recuperação e sem um depósito da chave no teu grupo, os backups encriptados não podem ser recuperados. É por isso que a visão geral mostra **Guarda o teu kit de recuperação** até o descarregares. Consulta [Externo e recuperação](offsite-recovery.md#recovery-kit). Gera uma chave com `openssl rand -hex 32` e guarda-a fora do servidor antes de depender de qualquer backup.

## Esqueci-me da palavra-passe de início de sessão

Escolhe **Esqueceste-te da palavra-passe?** na página de início de sessão. O BombVault escreve um código de 12 caracteres no registo do seu container (no Unraid: separador Docker, ícone do BombVault, **Logs**). Escreve-o com uma palavra-passe nova no prazo de 10 minutos. A reposição também desliga a autenticação de dois fatores, remove todas as passkeys, revoga todos os tokens de API, chaves MCP e inícios de sessão OAuth, termina todas as outras sessões e desliga o emparelhamento de telemóveis. Volta a adicionar as tuas passkeys em **Definições, Segurança**, liga aí de novo o emparelhamento de telemóveis e cria tokens e chaves novos onde ainda precisares deles. Consulta [Modelo de segurança](configuration.md#login).

## Os backups pararam e dizem que a chave já não pode ser aberta

Este servidor assumiu a chave de backup de outro servidor no Assistente de emergência, e a `APP_KEY` foi alterada depois. A chave assumida está selada com a APP_KEY antiga, por isso o BombVault não a consegue abrir e prefere não fazer backups a escrever com a chave errada. **Precisa de atenção** na visão geral indica-o. Repõe a APP_KEY no que era, ou carrega o kit de recuperação outra vez na página Restauração em **Emergência**.

## Um telemóvel vai parar à página de início de sessão

Um telemóvel emparelhado só recebe a sessão através do grupo enquanto **Os telemóveis podem emparelhar com as doze palavras do grupo** estiver ligado em **Definições, Segurança**. Uma reposição da palavra-passe desliga-o. Uma aplicação mais antiga do que o servidor também não recebe sessão até ser atualizada. Um servidor adicionado pelo endereço pede sempre a palavra-passe. Um telemóvel removido da página Instâncias regressa desligado quando a sua aplicação volta a abrir; liga **Consultar esta instância** em **Monitorização** nos seus detalhes para o deixar entrar.

## O backup de VM não se liga

O backup de VM comunica com o libvirt por SSH, nunca por uma montagem.

- Confirma que o SSH está ativado no host e que a chave pública do BombVault está autorizada em `/root/.ssh/authorized_keys` (Definições, Integrações, SSH do anfitrião mostra a chave e um botão **Testar ligação**).
- Numa rede `br0.x` personalizada, define `LIBVIRT_HOST` para o IP LAN do teu Unraid (o container não consegue alcançar o host via `host.docker.internal` aí). Ativa **Settings, Docker, Host access to custom networks**.
- Se alteraste a porta SSH do Unraid, define `LIBVIRT_SSH_PORT` para corresponder.
- No TrueNAS, define `LIBVIRT_URI` com o socket libvirt próprio do TrueNAS, dá à app a capability `SETPCAP` e adiciona a chave ao root em **Credentials, Users** (consulta [TrueNAS SCALE](getting-started.md#truenas)).
- O diagnóstico completo passo a passo (teste de alcance, encaminhamento de VLAN, `Permission denied (publickey)`, `Host key verification failed`) está no [guia de backup de VM por SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Um instantâneo a quente de VM não correu

Os instantâneos a quente precisam do agente convidado qemu instalado na VM e do disco em `/mnt/cache` (ou `/mnt/diskX`), não em `/mnt/user`. Numa VM desligada, o a quente recorre automaticamente ao ordenado. Um backup ordenado desliga a VM, faz backup dos discos, e depois reinicia-a, por isso é sempre consistente. Espera até dez minutos que o convidado se desligue e nunca o força a desligar, para que um convidado Windows a instalar atualizações não seja cortado a meio. Um convidado que ainda esteja a correr depois disso fica em paz, e esse backup falha com o motivo.

## Um backup falhou com "repository is already locked"

Isto é geralmente um bloqueio restic órfão deixado para trás quando o container foi atualizado ou reiniciado a meio de uma operação. O BombVault deteta um bloqueio comprovadamente órfão, força a sua limpeza e reexperimenta uma vez, automaticamente. Se persistir, abre o cartão **Verificar os backups** em **Definições, Verificação** e escolhe **Desbloquear** em **Mais** na linha da secção afetada para limpar um bloqueio preso à mão. Um problema genuíno continua a vir ao de cima em vez de ser escondido. Depois de um reinício, o BombVault espera até esse bloqueio passar dez minutos sem ser renovado. Um restic ainda em execução, por exemplo num segundo BombVault no mesmo repositório, renova o bloqueio a cada cinco minutos.

## A minha cópia externa não aconteceu após um backup

A replicação externa é de melhor esforço por conceção, por isso um percalço externo nunca faz o backup local falhar. Verifica quando o destino copia: o cartão **Copiar** diz **Depois de cada backup** ou indica um plano próprio, e **Definições, Agendamento, Cópias externas por secção** pode dar a uma secção um plano próprio. Usa **Replicar agora** no cartão **Cópia externa** da secção em **Definições, Destinos** para uma execução a pedido, e observa o cabeçalho de estado da visão geral, que diz **Cópia externa em curso** enquanto corre. Uma cópia falhada aparece em **Precisa de atenção**.

## O certificado de um rest-server não é de confiança

O restic recusa um certificado que não conhece, e o erro diz onde o colocar. Copia o certificado autoassinado do rest-server, ou o da autoridade que o assinou, como ficheiro PEM para a pasta `certs` da pasta de dados do BombVault (`/config/certs` no container, `appdata/bombvault/certs` no Unraid) e testa de novo. Não é preciso reiniciar.

## Um backup diz que os dados fora de Host Data não foram copiados {#data-outside-host-data}

O container guarda dados numa pasta que o BombVault não consegue ler através de Host Data. O backup guarda a definição e todas as pastas a que chega, pinta de laranja a sua linha no registo de atividade e indica as pastas que deixou de fora. Há dois casos comuns: um ixVolume no TrueNAS em `/mnt/.ix-apps` e um volume do Docker com nome em `/var/lib/docker/volumes`, como o volume da base de dados de uma stack compose. O cartão **Pastas a copiar** do separador **Definições** do container mostra cada uma dessas pastas com o caminho dentro do container e o caminho no host, marcada a vermelho.

Um volume anónimo do Docker não é reportado. O Docker cria-o sozinho para uma pasta que a imagem declara e que ninguém mapeou, dá-lhe um nome de 64 dígitos hexadecimais e guarda-o em `/var/lib/docker/volumes`. Normalmente contém uma cache, por isso o backup fica verde e o cartão apenas o mostra, com uma nota de que é anónimo.

- **Queres copiar esses dados.** Mapeia a pasta para um caminho do host abaixo de Host Data, por exemplo uma pasta da tua partilha appdata, e o próximo backup inclui-a.
- **Não precisas deles.** Adiciona um dos seus dois caminhos como uma linha no cartão **Padrões de exclusão** do mesmo separador: o caminho dentro do container, como `/cache`, ou o caminho no host que o aviso indica, como `/var/lib/docker/volumes/jellyfin_cache/_data`. A partir do próximo backup, o aviso deixa essa pasta de fora, e a pré-visualização por baixo do campo diz porquê. Uma linha para uma pasta acima também serve, um nome simples ou `/` não.

## Um backup aparece como Adiado {#deferred-backups}

A app do container estava ocupada, por isso o seu backup espera e volta a ser tentado sozinho. A linha no registo de atividade diz porquê e quando é a próxima tentativa; o cartão **Quando a app está ocupada** no separador **Definições** do container diz o mesmo e quando termina a espera. Não é preciso fazer nada. Para fazer o backup já, usa **Fazer backup agora** assim que a app estiver livre.

- **A tua própria verificação.** Faz o **Comando pré-backup** terminar com o código 75 enquanto a app está ocupada, por exemplo `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Qualquer outro código diferente de 0 faz falhar o backup como antes, por isso um script avariado continua a aparecer em **Precisa de atenção**.
- **O Plex, o Jellyfin ou o Emby nunca é consultado.** O BombVault pergunta primeiro ao servidor no endereço do próprio container e, quando aí não há resposta, corre curl ou wget dentro do container. Uma imagem sem nenhum dos dois não pode ser consultada através de si mesma; nesse caso o registo indica-o. Com **Fazer backup quando o servidor não responde** ligado, um backup assim corre na mesma; com a opção desligada, espera.
- **O Jellyfin ou o Emby recusa.** Verifica a **Chave de API**. Cria-se no painel do servidor, em chaves de API.
- **O Plex recusa.** O BombVault lê o token do `Preferences.xml` na pasta montada em `/config`. Um servidor onde nunca se iniciou sessão não tem token e só responde onde permite acesso sem ele.
- **Um filme em pausa deixa o backup à espera a noite toda.** Define **Ignorar sessões em pausa após (min)**, por exemplo para 30.
- **A verificação de tráfego nunca deixa passar o backup.** Sobe **Ocupado a partir de (KiB/s)** acima do que o container envia quando ninguém o usa. Logo depois de o BombVault arrancar, a verificação espera até ter observado o container durante **Calmo durante (min)** minutos.
- **O backup nunca corre à noite.** Com uma janela definida, as tentativas só acontecem dentro dela. Uma espera que chega ao prazo fora da janela termina na mesma no prazo.

## Um backup grande foi terminado por falta de memória

Define **Limite de memória por backup (MiB, 0 = nenhum)** no cartão **Desempenho** em **Definições, Sistema**, bem abaixo do que o container pode usar. Perto do limite, o restic liberta memória mais vezes, o que custa alguma velocidade, por isso o backup abranda em vez de ser terminado.

## Um restauro abortou antes de começar

Antes de qualquer coisa ser parada ou removida, o restauro corre uma verificação de conflitos pré-voo: verifica que o IP estático do container e as portas do host publicadas estão livres. Se outro container já detém uma, aborta com uma mensagem clara e acionável em vez de deixar um restauro a meio. Liberta a porta ou o IP em conflito, e depois reexperimente.

Se a rede do container não existir, por exemplo porque a sua app do TrueNAS foi apagada, a verificação antes do restauro indica a rede e o restauro para antes de alterar o que quer que seja. Reinstala a app no TrueNAS com o mesmo nome, o que volta a criar a rede, e depois restaura os seus dados.

## Uma exportação simples falhou em vez de escrever um ficheiro

Se **Cifrar exportações com age** estiver ligado em **Definições, Sistema** mas não estiver definido nenhum destinatário válido, uma exportação falha com um erro claro em vez de escrever texto simples. Adiciona um destinatário válido (uma chave pública age ou uma chave pública SSH), ou desliga a encriptação se pretenderes que a exportação seja texto simples. Consulta [Funcionalidades](features.md).

## Um dump de base de dados falhou

Um dump falhado nunca faz falhar o backup à volta dele; fica registado como uma execução falhada própria, e o motivo diz o que corrigir.

- **Entrada recusada.** O dump entra com as variáveis de palavra-passe do próprio container (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` ou as versões `_FILE`). Confirma-as no container da base de dados. Uma variável `_FILE` que aponta para um segredo que o utilizador do container não consegue ler falha da mesma maneira.
- **Privilégios em falta.** Com uma palavra-passe de root aleatória, o dump só consegue entrar como utilizador da aplicação, pelo que contém apenas aquela base, e o MySQL 8.4 e mais recentes podem recusá-lo de todo. Dá ao container uma palavra-passe de root a sério, ou desliga-lhe o dump.
- **As tabelas de sistema precisam de atualização.** O MariaDB recusa o dump quando as suas tabelas de sistema vêm de uma versão mais antiga (erro 1558). Acrescenta a variável `MARIADB_AUTO_UPGRADE=1` e reinicia o container, ou corre `mariadb-upgrade` lá dentro uma vez.
- **Sem ferramenta de dump.** Uma imagem enxuta ou feita à mão sem `pg_dump`, `mysqldump` ou `mariadb-dump` não pode ser despejada. Usa a imagem oficial, ou desliga o dump.
- **Um limite de tempo.** Um dump tem `DB_DUMP_MAX_HOURS` (6 por omissão), o backup à volta tem `BACKUP_MAX_HOURS`, e um dump que deixa de avançar é cortado ao fim de `BACKUP_STALL_HOURS`. Este último caso vem quase sempre de um bloqueio que a aplicação mantém. Sobe o limite que disparou, ou faz o dump enquanto a aplicação está sossegada.
- **O container está em pausa ou a reiniciar.** O dump fala com o servidor em funcionamento. Se o container reinicia sem parar, o seu próprio registo diz porquê.
- **Um dump danificado não pôde ser removido.** Um dump que o BombVault não conseguiu terminar é apagado. Quando esse apagamento falha, o dump fica na lista marcado como danificado e podes eliminá-lo aí.

## Uma importação falhou

Uma importação para o container, põe a pasta de dados de lado e deixa a imagem criar uma vazia no lugar. Se falhar um passo antes da importação em si, a pasta antiga volta sozinha ao sítio. Se falhar a importação, o container fica com a pasta nova e a antiga permanece ao lado como `<pasta de dados>.bombvault-before-import-<data e hora>`; a mensagem de erro da execução indica o caminho exato.

Uma pasta de dados que é um conjunto de dados próprio, como no TrueNAS, não pode mudar de nome. A importação deixa então a pasta no lugar e move o seu conteúdo para a pasta guardada ao lado, por isso o conjunto de dados acima precisa de espaço para ele. A importação verifica isso antes de parar o container.

Para a repor à mão: para o container, muda o nome da pasta de dados atual para a tirar do caminho, muda o nome da pasta guardada de volta ao original e arranca o container. Num conjunto de dados, move o conteúdo em vez de mudar o nome das pastas. No Unraid, o gestor de ficheiros no separador Shares faz isto.

## Uma cópia de um conjunto de dados ZFS falhou ou ignorou um conjunto {#zfs-datasets}

Cada problema tem um código de motivo entre parênteses retos, e a página [Conjuntos de dados ZFS](zfs-datasets.md#reason-codes) lista-os todos com a solução. Os três mais comuns:

- **`snapshot-loop`**: o instantâneo não chegou ao BombVault porque o Host Data não passa as novas montagens. Edita o container, põe o Access Mode do Host Data em Read/Write - Slave e reinicia o BombVault.
- **`key-not-loaded`**: um conjunto cifrado cuja chave não está carregada é ignorado. Carrega a chave com `zfs load-key` e monta o conjunto; o próximo backup inclui-o.
- **`ssh-auth`**: o servidor recusou a chave do BombVault. **Ligação ZFS a este servidor**, que se abre com **Ver ligação** na página Backups, mostra o comando que a autoriza; executa-o uma vez no servidor.

## Uma entrada fica em "A aprender N/10"

A maioria das verificações de anomalias começa após 10 backups bem-sucedidos de uma entrada, e a contagem recomeça após **Marcar como esperada** e depois de a seleção da entrada mudar. Uma entrada sem agendamento não aprende, e um container sem appdata não tem com que aprender, como indica o seu distintivo. **Progresso da aprendizagem** na página Anomalias mostra um anel de dez partes por entrada.

## A retenção deixou de apagar os backups antigos de uma entrada

Uma anomalia crítica aberta está a segurá-los: a origem da entrada está quase vazia, encolheu muito, ou um backup voltou a guardar a maior parte dos dados. Abre a anomalia a partir do distintivo da entrada. Se faltarem dados ou tiverem sido cifrados, restaura primeiro a partir do último backup bom indicado. Depois confirma a anomalia, ou marca-a como esperada se a alteração foi tua, e a execução seguinte limpa como de costume. A pré-visualização da retenção assinala essa entrada como mantida. Numa entrada ZFS só o conjunto de dados indicado na anomalia mantém os backups antigos; os outros conjuntos da árvore são limpos como de costume.

## A limpeza manual diz que algumas entradas foram mantidas

A mesma causa: a limpeza deixa em paz os backups antigos de uma entrada com uma anomalia destas e indica-a na sua mensagem. Tudo o resto é limpo como de costume.

## A importação do histórico diz que um repositório não pôde ser lido

Depois da atualização, o BombVault lê uma vez o tamanho dos backups anteriores de cada repositório. Um repositório que não estava acessível nesse momento, como um destino externo em baixo ou uma partilha não montada, aparece no cartão **Anomalias** em **Definições, Verificação** e é tentado de novo uma vez por dia. Entretanto, as suas entradas aprendem com os backups novos.

## O aviso de espaço em disco não coincide com o painel do Unraid

Na partilha de utilizador do Unraid (`/mnt/user`) o espaço livre é o de todo o array, não o de um disco. Os repositórios remotos só são medidos através de remotes rclone que indicam o seu espaço livre; os repositórios S3, B2, REST e SFTP não têm valor e aparecem como não medidos no cartão **Anomalias**.

## Um assistente de IA não consegue ligar-se

A página [Servidor MCP](mcp.md#troubleshooting) explica o que significa cada código de estado e cada recusa do ponto de ligação MCP, e o que fazer.

## O container continua a reiniciar ou parece não-saudável

O BombVault reporta saudável/não-saudável a partir do seu próprio `/api/health`. Uma ferramenta de auto-recuperação (como o Autoheal) pode reiniciá-lo automaticamente se o motor alguma vez encravar. Verifica o registo do container e o cartão **Integração com o host** para a causa subjacente.

## Ainda preso?

- Lê as páginas completas de [Configuração](configuration.md) e [Externo e recuperação](offsite-recovery.md).
- Pergunta na [thread de suporte do Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Abre uma [issue no GitHub](https://github.com/junkerderprovinz/bombvault/issues).
