# Externo e recuperação

!!! note "As cópias externas esperam depois de uma reconstrução"
    Quando o assistente de emergência ou **Descobrir backups** reconstrói entradas sem as definições antigas, as cópias externas dessas secções ficam em pausa até a predefinição delas ser confirmada. Vê [Depois de uma reconstrução](#after-a-rebuild).

Os backups locais protegem-te de um container perdido ou de uma má atualização. As cópias externas e um kit de recuperação testado protegem-te da máquina inteira, de ransomware ou de um incêndio. Esta página cobre os destinos e as cópias externas, tornar uma cópia à prova de adulteração, provar que consegues restaurar, emparelhar vários servidores e recuperar tudo quando o próprio BombVault desapareceu.

## Cópias externas {#off-site-copies}

Mantém o backup local rápido e junta um ou mais destinos fora de casa. O BombVault copia para lá os novos instantâneos com `restic copy` numa base de melhor esforço, por isso um percalço externo nunca faz o backup local falhar. Nesta forma, cada secção escreve primeiro na sua própria pasta e o repositório externo é uma cópia. A pasta própria não tem de estar neste servidor: vê [A pasta própria de uma secção](#own-folder) para a manter num destino como um S3 ou um rest-server.

- **Vários destinos por secção.** Cada secção (containers, VMs, flash, auto-backup, pastas e conjuntos de dados ZFS) pode copiar para vários destinos externos ao mesmo tempo, para poderes manter, por exemplo, um rest-server em casa de um amigo e um bucket S3 em paralelo. O cartão **Destinos por secção** na página Destinos liga ou desliga cada linha de uma secção para um destino.
- **Quando as cópias correm.** O cartão **Copiar** de um destino define **Quando copiar**: **Depois de cada backup**, ou segundo um plano próprio que reúne os instantâneos e copia a horas fixas. Em **Definições, Agendamento, Cópias externas por secção**, **Exceção** dá a uma secção o seu próprio plano de cópia.
- **O cartão Cópias externas.** Em **Definições, Destinos**, cada secção com uma cópia externa tem uma linha em **Cópias externas**, com **Testar ligação**, **Replicar agora** para iniciar uma cópia a pedido, e **Configurar** ou **Editar**. O resto das definições de cópia da secção abre-se na sua janela a partir daí.
- **A retenção** define-se por destino em **Durante quanto tempo guardar**, para poderes manter as cópias externas mais tempo como arquivo. A cópia no campo externo de uma secção segue-a como todas as outras cópias, e uma secção pode manter valores próprios em **Regras por secção**. Tudo a zero nunca apara os instantâneos externos. Vê [Durante quanto tempo guardar](#keep).
- **Os limites de largura de banda** (**Largura de banda externa** em **Definições, Destinos**, ou o cartão **Copiar** de um destino) limitam a taxa de envio e receção do restic para que as cópias não saturem a tua WAN. Introduzem-se em KiB/s, a unidade que o restic aceita, e cada campo mostra a mesma taxa em MB/s e Mbit/s enquanto escreves: 2048 lê-se como cerca de 2,0 MB/s (16,8 Mbit/s).
- **A compressão** é **Desligado**, **Automático** (a predefinição do restic) ou **Máximo**, definida em cada destino e, para uma secção, nas suas regras aí. Os backups, as cópias e a poda escrevem com ela, e o kit de recuperação nomeia-a, para que o restic simples possa continuar a escrever da mesma maneira.
- **Primeiro o streaming.** Enquanto um servidor multimédia como o Plex, o Jellyfin ou o Emby transmite, as cópias enviam com um limite mais baixo e voltam ao normal alguns minutos depois de o streaming acabar. O cartão **Streaming** na página Destinos tem **Abrandar cópias externas durante o streaming**, **Servidores multimédia** (containers com Plex, Jellyfin ou Emby no nome da imagem até escolheres), **Streaming acima de (Mbit/s)**, **Limite de envio durante o streaming (KiB/s)** e **Voltar ao normal após (min)**. O BombVault lê o tráfego do Docker, por isso um servidor multimédia na rede do host não pode ser medido. REST, S3, B2, Azure, Google Cloud, Swift e rclone sobre HTTP abrandam a meio de uma cópia. O SFTP e as pastas locais ou montadas adotam o limite mais baixo no passo de cópia seguinte.
- **Armazenamento frio (S3).** Um destino S3 pode usar uma **Classe de armazenamento** mais barata que o restic ainda consegue ler de imediato, como Standard-IA, One Zone-IA, Intelligent-Tiering ou Glacier Instant Retrieval. O Glacier Flexible Retrieval e o Deep Archive ficam de fora, porque o restic não consegue restaurar a partir deles. Um remote rclone define a sua classe na configuração do rclone.
- **Orçamento de crescimento.** O **Orçamento de crescimento (GB, 0 = desativado)** na janela de cópia de uma secção dispara um alarme quando o repositório cresce para além desse tamanho. Nunca elimina nada, por isso serve para uma cópia append-only que nada poda a partir daqui.
- Enquanto uma cópia corre, o cabeçalho de estado da visão geral diz **Cópia externa em curso**. É um indicador ativo, não uma barra de percentagem, porque o `restic copy` não expõe progresso legível por máquina.

!!! note "Restaurar de qualquer local"
    Cada entrada lista os seus backups como uma única linha cronológica por todos os lugares onde um backup está, no separador **Backups**. Um backup copiado para o B2 aparece uma vez, marcado com cada lugar que o tem. Uma restauração usa o primeiro lugar a que consegue chegar, começando pelo repositório para onde a entrada escreve, e podes escolher outro lugar por instantâneo. Os destinos externos só são lidos quando clicas em **Verificação**, porque listar armazenamento na nuvem demora um pouco. Eliminar num só lugar verifica primeiro os outros e diz se era a última cópia. Vê [Restauração](restore.md).

## Destinos {#destinations}

Um destino é um lugar para onde vão os backups: uma pasta local, uma partilha, um servidor teu ou um serviço de nuvem. Configura-se uma vez e depois serve todas as secções. Abre **Destinos** a partir do respetivo mosaico de definições, ou liga lá **Mostrar também na barra lateral** para o fixar.

![A página Destinos: a lista de destinos e o fluxo de que secção vai para onde.](assets/screenshots/storage/locations-dark.png#only-dark)
![A página Destinos: a lista de destinos e o fluxo de que secção vai para onde.](assets/screenshots/storage/locations-light.png#only-light)

**Adicionar destino** abre uma janela em quatro passos:

1. **Fornecedor.** Cada serviço está listado com o seu logótipo, com uma pesquisa no canto superior direito da janela, em grupos: **Local** para um disco ou partilha ligados a este servidor, **NAS ou servidor** (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV e outros), **Serviço S3** (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 e outros), o teu próprio servidor S3 (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), **Armazenamento na nuvem** (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud e o resto que o rclone suporta) e **Do teu grupo**, os servidores recetores do teu grupo. Cada um diz quão bem serve para backups: os discos na nuvem abrandam com muitos pedidos, por isso o primeiro backup e a poda demoram mais aí. Para um serviço que nenhum mosaico cobre, o mosaico **Rclone** em **Configuração própria do rclone** aceita remotes colados de um rclone.conf.
2. **Entrar.** Os campos dependem do serviço: uma chave de acesso para S3, utilizador e palavra-passe para WebDAV e SMB, uma palavra-passe de aplicação onde o início de sessão de dois fatores bloqueia a normal, a chave SSH pública do BombVault para SFTP e a Storage Box, ou um token para serviços que iniciam sessão por um navegador. Para esses, a janela mostra um comando `rclone authorize` para correr num computador com navegador, e o token que ele imprime vai para o campo. Onde já tens [credenciais guardadas](#sign-ins) para o serviço, **Credenciais guardadas** escolhe-as em vez de **Novas chaves**. **Testar ligação** verifica o início de sessão antes de se guardar o que quer que seja. A janela lista então as pastas do destino, com **Nova pasta** para criar uma e o espaço livre onde o serviço o indica. Uma pasta vazia é o mais seguro. O BombVault põe lá dentro uma pasta por secção.
3. **Proteção.** A janela diz com clareza o que o serviço consegue. Um rest-server em modo append-only recusa a eliminação, e o teste de adulteração verifica-o. Um bucket S3 pode manter versões antigas com versionamento e bloqueio de objetos, o que o BombVault ainda não consegue verificar. Um disco na nuvem não consegue recusar a eliminação de todo: quem entra no servidor entra também nessa cópia. Liga **Proteção contra eliminação (append-only)** só onde o outro lado recusa mesmo a eliminação. O BombVault nunca poda aí. **Para uma emergência** diz o que o kit de recuperação guarda para este destino.
4. **Para que secções.** Escolhe as secções que fazem backup para aqui. A primeira cópia leva o histórico todo. As entradas que escolheram destinos próprios não recebem o novo por si, e a janela nomeia-as.

![Adicionar destino: o primeiro passo, com cada fornecedor como mosaico.](assets/screenshots/storage/add-providers-dark.png#only-dark)
![Adicionar destino: o primeiro passo, com cada fornecedor como mosaico.](assets/screenshots/storage/add-providers-light.png#only-light)

![Adicionar destino: o passo de início de sessão depois de um teste bem-sucedido.](assets/screenshots/storage/add-signin-dark.png#only-dark)
![Adicionar destino: o passo de início de sessão depois de um teste bem-sucedido.](assets/screenshots/storage/add-signin-light.png#only-light)

Alguns serviços pedem um pouco mais no início de sessão:

- **SFTP e a Storage Box.** **Adicionar chave** coloca a chave pública do BombVault no servidor com a **Palavra-passe da conta**, que serve só para esse início de sessão e não fica guardada em lado nenhum. Por baixo aparece o comando para o fazer à mão. O teste mostra a **Chave do servidor** com a sua impressão digital. O BombVault memoriza-a e recusa o servidor se mais tarde mostrar uma diferente.
- **As chaves S3 para um único bucket**, como um token de objetos do Cloudflare R2 ou uma chave AWS restrita, podem não ter permissão para listar os outros buckets. Escreve o bucket em **Bucket**, e o teste e a lista de pastas abrem esse bucket.
- **Regiões e endereços S3.** A região é lida de um endpoint que a nomeia (AWS, Backblaze B2, Hetzner, Scaleway, OVH, Wasabi), o Garage começa com a sua própria região predefinida, e o IDrive e2 pede o endpoint que o seu painel mostra. Um endereço colado com o bucket, como o painel do R2 o mostra, preenche o servidor e o bucket. Um bucket criado a partir do navegador de pastas fica na região que o formulário indica.
- **Classe de armazenamento.** O seletor oferece só as classes que o serviço aceita, e **Mais definições** mostra só as opções que moldam um bucket novo.
- **Google Drive e OneDrive.** Os dados podados são eliminados de vez em vez de irem para o lixo ou a reciclagem, onde continuariam a contar para a quota. Uma unidade partilhada do Google fica em **Mais definições**.
- **Os servidores SMB que não listam as suas partilhas** recebem o nome de uma em **Partilha**, e o teste abre essa partilha diretamente.
- **FTP** pede a sua **Encriptação**: **Nenhuma**, **TLS explícito (FTPES)** ou **TLS implícito (FTPS, porta 990)**.
- **OneDrive e Zoho WorkDrive.** Quando um início de sessão abre várias unidades ou espaços de trabalho, a janela pergunta para qual vão os backups.
- **Um rest-server com um certificado autoassinado.** Coloca o certificado como ficheiro PEM na pasta `certs` da pasta de dados do BombVault (`/config/certs` no container, `appdata/bombvault/certs` no Unraid). Os backups, as cópias e o teste da proteção contra eliminação confiam nele sem reiniciar.

Os serviços S3 passam pelo backend S3 do próprio restic, o que permite aplicar uma classe de armazenamento e o bloqueio de objetos. Um rest-server é alcançado diretamente pelo restic. Todos os outros serviços passam pelo rclone que o BombVault traz, e a sua secção da configuração do rclone aparece no cartão **rclone.conf** na página do destino, com os segredos ocultos. Uma exportação de definições leva os destinos e, com as credenciais incluídas, leva também os seus inícios de sessão.

Guardar um destino que já tem backups de containers, VMs ou pastas, por exemplo um que removeste e voltas a adicionar, junta-o a essas secções como uma cópia por marcar. Nada é copiado para lá até a marcares, mas a **Restauração** oferece logo os backups antigos.

A pasta neste servidor chama-se **Local** em todo o lado: nas páginas de armazenamento, no fluxo, numa entrada e na Restauração. Com várias pastas neste servidor, cada uma tem o seu nome. Um destino que o campo externo de uma secção configura recebe o nome do seu lugar, a última pasta do seu endereço ou, sem ela, o seu servidor, e ganha um número quando outro destino já tem esse nome. Todos os destinos podem ser renomeados no campo **Nome** do seu cartão **Ligação**, também uma pasta de bucket que várias secções partilham e a cópia que o campo externo de uma secção descreve, e um nome que lhe dês fica. Onde dois destinos com o mesmo nome estão lado a lado, cada um leva o seu servidor ou pasta.

### A página de um destino {#location-page}

Cada destino tem uma página própria. Mostra só os cartões que se aplicam a esse destino, por esta ordem:

![A página de um destino: Pastas próprias, Ligação e Credenciais.](assets/screenshots/storage/location-page-dark.png#only-dark)
![A página de um destino: Pastas próprias, Ligação e Credenciais.](assets/screenshots/storage/location-page-light.png#only-light)

- **Pastas próprias**: as secções que escrevem aqui primeiro os seus backups. As credenciais, a proteção e os limites deste destino aplicam-se-lhes. **Usar como pasta própria** muda para aqui a pasta própria de mais secções, e **Alterar** afasta uma outra vez. Vê [A pasta própria de uma secção](#own-folder).
- **Guarda diretamente aqui**: cada secção cujas entradas fazem backup diretamente para este lugar em vez de para a sua pasta própria, e quantas entradas o fazem. Um repositório direto destes recebe todas as suas definições deste destino. Um para onde já nada faz backup di-lo, e **Remover** tira-o da lista. Os backups no seu lugar ficam. Vê [Repositórios diretos](#direct-repositories).
- **Ligação**: o **Nome**, o interruptor **Disponível** (desligado significa que o BombVault deixa de escrever aqui, e as entradas que fazem backup aqui falham com uma mensagem clara em vez de irem parar a outro sítio), a pasta, a **Compressão**, a **Classe de armazenamento** e **Fora das instalações**. No destino **Local** lista a pasta de cada secção em **Pastas próprias aqui**. **Alterar a ligação** testa e guarda um novo endereço ou início de sessão. Quando outros lugares iniciam sessão com as mesmas credenciais guardadas, a janela di-lo, e uma alteração aqui dá a este destino credenciais próprias em vez de reescrever as partilhadas.
- **Credenciais**: que credenciais guardadas usa (**Entra com**), que outros destinos as usam também (**Também usadas por**), e **Editar**, que abre a janela das credenciais. Vê [Credenciais](#sign-ins).
- **rclone.conf**: a secção deste destino da configuração do rclone, para um destino que passa pelo rclone.
- **Proteção contra eliminação**, com **Proteção contra eliminação (append-only)**, **Aceitar sem proteção contra eliminação**, **Testar agora a proteção contra eliminação** e **Quem poda**, que diz se o BombVault poda a cópia e as pastas próprias aqui, pelo cartão **Durante quanto tempo guardar** deste destino ou pelos valores próprios de uma secção em **Regras por secção**, ou deixa isso ao outro lado. Uma pasta local só mostra este cartão quando tem uma proteção que se pode desligar. Vê [Proteção contra eliminação](#delete-protection-append-only).
- **Copiar**: **Quando copiar**, e o **Limite de envio (KiB/s)** e o **Limite de receção (KiB/s)**. Um destino que guarda só pastas próprias e nenhuma cópia mostra os limites num cartão chamado **Largura de banda**.
- **Durante quanto tempo guardar**: **Curto**, **Equilibrado**, **Longo** ou **Valores próprios**, com uma linha cronológica dos backups que a regra mantém. Vê [Durante quanto tempo guardar](#keep).
- **Já está sob este destino**: secções cujo endereço escrito à mão está numa pasta deste destino. Assumir uma deixa o repositório onde está. As credenciais, a classe de armazenamento e a proteção contra eliminação passam então a vir do destino. O BombVault verifica primeiro que as credenciais do destino abrem o repositório, e recusa pôr um repositório append-only sob um destino que não é append-only.
- **Regras por secção**: o repositório próprio, a retenção ou a compressão de uma secção neste destino. **Alterar regras** abre-as, e **Repor o do destino** devolve uma secção ao destino. As credenciais, a largura de banda e quando as cópias correm vêm sempre do destino.
- Um recipiente que mostra quão cheio está o destino, com a marca de onde está cheio e quantas semanas faltam ao ritmo de crescimento das últimas semanas. Um destino que não indica o seu tamanho diz **Tamanho desconhecido**.

**Mover** e **Remover destino** estão entre as ações da página, descritos em [Mover e remover](#move-remove).

Um destino remoto que guardava backups e de repente não tem repositório, como um rest-server cujo disco não montou, não recebe sozinho um novo, vazio. A sua página di-lo assim que um backup ou uma cópia encontra o repositório desaparecido. Monta o disco outra vez. Se esvaziaste o repositório antigo de propósito, **Criar aqui um repositório novo** recomeça, para todas as secções do destino e dos seus repositórios de uma vez.

Um servidor recetor que outra instância do teu grupo executa aparece entre os fornecedores como **Do teu grupo**. Vê [Servidor recetor](#receiving-server).

O BombVault verifica os seus destinos em cada arranque. Corrige o que não precisa de nenhuma decisão tua, como uma ligação a uma cópia ou repositório que desapareceu, uma cópia na pasta própria de uma secção, ou um repositório direto cuja cópia desapareceu, e escreve uma linha no registo por cada correção. O que não tem correção segura é apenas registado.

![A mesma página num telemóvel.](assets/screenshots/storage/location-page-phone-dark.png#only-dark)
![A mesma página num telemóvel.](assets/screenshots/storage/location-page-phone-light.png#only-light)

### Durante quanto tempo guardar {#keep}

Durante quanto tempo os backups são guardados define-se por destino, no seu cartão **Durante quanto tempo guardar**. A regra cobre as cópias nesse destino e as pastas próprias que lá estão, o destino **Local** incluído. Uma secção pode manter valores próprios num destino em **Regras por secção**, e esses ganham ao cartão do destino para essa secção.

Uma regra guarda um backup por dia, semana, mês e ano, tão para trás quanto definires, e limpa o resto depois de cada backup ou cópia:

| Predefinição | Últimos backups | Dias | Semanas | Meses | Anos |
|---|---|---|---|---|---|
| **Curto** | 0 | 7 | 4 | 3 | 0 |
| **Equilibrado** | 0 | 7 | 4 | 6 | 2 |
| **Longo** | 0 | 14 | 8 | 12 | 3 |

**Valores próprios** define tu mesmo as cinco contagens. Um 0 desliga essa contagem, e uma regra com todas as contagens a 0 guarda tudo. Os pontos no cartão mostram quais de 40 backups de exemplo uma regra guarda, o mais recente na ponta direita. A amostra é a mesma para todos os destinos, já que a imagem é desenhada antes de o BombVault ter lido os backups lá. Conta para trás a partir de hoje, por isso o número de pontos que ficam muda com a data.

**Perguntar antes de guardar menos**, ligado por predefinição, pergunta antes de se guardar uma alteração que guarda menos backups, e diz quantos dos 40 backups de exemplo a nova regra remove. Aplica-se a todos os destinos. **Repor o do destino** em **Regras por secção** faz a mesma pergunta.

Um destino com proteção contra eliminação nunca é podado a partir desta máquina. O outro lado guarda ou larga os backups antigos segundo as suas próprias regras. Enquanto uma entrada tem uma anomalia crítica em aberto sobre dados perdidos, a retenção mantém os backups antigos dessa entrada até a confirmares. Vê [Anomalias](anomalies.md).

Para ver o que a próxima execução apagaria de verdade, abre **Pré-visualizar** em **Definições, Destinos**. Lista **O que a próxima execução iria remover**, por repositório e por entrada, com os pontos de restauro nomeados. Não bloqueia nada e não muda nada.

![Durante quanto tempo guardar com Equilibrado escolhido: os pontos mostram quais dos 40 backups de exemplo ficam.](assets/screenshots/storage/keep-dark.png#only-dark)
![Durante quanto tempo guardar com Equilibrado escolhido: os pontos mostram quais dos 40 backups de exemplo ficam.](assets/screenshots/storage/keep-light.png#only-light)

#### Exemplo: o que guarda o Equilibrado {#example-retention}

Um container tem feito backup todas as noites às 03:00 desde 1 de outubro de 2024, por isso na quarta-feira 4 de novembro de 2026 já tem 765 backups. O seu destino está em **Equilibrado**: 7 dias, 4 semanas, 6 meses e 2 anos.

Cada contagem guarda o backup mais recente do seu dia, semana, mês ou ano, e um backup que várias contagens escolhem é guardado uma só vez. As semanas vão de segunda a domingo:

| Regra | Backups que escolhe | Novos que acrescenta |
|---|---|---|
| 7 dias | 4 nov, 3 nov, 2 nov, 1 nov, 31 out, 30 out, 29 out | todos os 7 |
| 4 semanas | 4 nov, 1 nov, 25 out, 18 out | 25 out, 18 out |
| 6 meses | 4 nov, 31 out, 30 set, 31 ago, 31 jul, 30 jun | 30 set, 31 ago, 31 jul, 30 jun |
| 2 anos | 4 nov, 31 dez 2025 | 31 dez 2025 |

Ficam 14 backups em 765. O mais antigo é de 31 de dezembro de 2025, por isso hoje podes recuar um pouco mais de dez meses. Graças à deduplicação ocupam pouco mais espaço do que um backup completo mais o que mudou pelo meio.

Os **Anos** merecem um segundo olhar. Contam os anos civis em que há um backup, a começar pelo atual, e o backup mais recente deste ano é o de hoje. Assim, o primeiro ano não acrescenta nada às outras contagens, e o segundo acrescenta o último backup do ano anterior. É por isso que o alcance de uma regra varia ao longo do ano. Os mesmos backups com cada predefinição:

| Predefinição | Backups guardados hoje | Backup mais antigo hoje | Alcance ao longo do ano |
|---|---|---|---|
| **Curto** | 10 | 30 de setembro de 2026 | 1 a 2 meses |
| **Equilibrado** | 14 | 31 de dezembro de 2025 | cerca de 4 meses no início da primavera, um ano inteiro a 31 de dezembro |
| **Longo** | 30 | 31 de dezembro de 2024 | 1 ano a 1 de janeiro, 2 anos a 31 de dezembro |

Passar de **Equilibrado** para **Curto** pergunta primeiro, por causa de **Perguntar antes de guardar menos**. A pergunta diz quantos dos 40 backups na imagem a nova regra elimina. Esse número vem da amostra, por isso muda com a data e não conta os teus próprios backups. Dos backups deste exemplo, iam 31 de agosto, 31 de julho, 30 de junho e 31 de dezembro de 2025. **Guardar mesmo assim** aplica a regra, e a poda seguinte elimina-os desse destino de vez.

Para manter os containers mais tempo do que o resto neste destino, carrega em **Alterar regras** em **Regras por secção**, escolhe **Containers** e define **Valores próprios** em **Regras para Containers**, por exemplo 14 dias, 8 semanas, 12 meses e 2 anos. As outras secções continuam a seguir o cartão do destino, e **Repor o do destino** devolve os Containers a ele.

### Credenciais {#sign-ins}

Umas credenciais são a chave ou palavra-passe que o BombVault usa para um serviço S3 ou um rest-server. As credenciais formam uma só lista, o cartão **Credenciais** em **Definições, Destinos**, e cada linha diz que destinos as usam (**Usadas por**).

- **Guardadas quando juntas um destino.** Adicionar destino guarda as chaves que escreves como credenciais. O destino seguinte no mesmo serviço pode escolhê-las em **Credenciais guardadas** em vez de **Novas chaves**, e uma alteração posterior aplica-se a todos os destinos que as usam.
- **A predefinida.** Uma estrela marca as credenciais predefinidas. São usadas onde não se escolhe nenhuma, como um endereço escrito à mão, uma origem de recolha, ou uma recuperação numa instalação nova. **Tornar padrão** dá a estrela a outras.
- **Editar e Testar.** **Editar**, aqui ou no cartão **Credenciais** de um destino, abre uma só janela para as credenciais. **Testar** inicia sessão em todos os destinos que as usam com o que escreveste, antes de se guardar o que quer que seja. **Guardar** grava-as.
- **Remover.** Umas credenciais só podem ser removidas quando nenhum destino ou origem de recolha as usa, e as predefinidas só saem como as últimas. A linha diz então o que ainda as usa.
- **Mudar a ligação de um destino** nunca reescreve credenciais que outros lugares partilham. O destino recebe antes credenciais próprias, e a janela diz de antemão que as suas credenciais são partilhadas. Para mudar as partilhadas em todo o lado, edita-as no cartão **Credenciais**.

![O cartão Credenciais: cada credencial guardada com o que a usa, e a estrela na predefinida.](assets/screenshots/storage/signins-dark.png#only-dark)
![O cartão Credenciais: cada credencial guardada com o que a usa, e a estrela na predefinida.](assets/screenshots/storage/signins-light.png#only-light)

Uma pasta neste servidor não precisa de credenciais. O SMB, o WebDAV, o SFTP, a Storage Box e os discos na nuvem guardam as suas na configuração do rclone, que o cartão **rclone.conf** do destino mostra, por isso não aparecem nesta lista.

### SMB, WebDAV e outros remotes do rclone {#smb-webdav}

Uma partilha Windows ou Samba e um servidor WebDAV (Nextcloud, ownCloud, SharePoint ou qualquer outro) são mosaicos de fornecedor em **Adicionar destino**, em **NAS ou servidor**. Preenche o host e a partilha (SMB) ou o endereço e o tipo de servidor (WebDAV), o utilizador e a palavra-passe, e o BombVault escreve a secção do rclone por ti. O rclone ofusca a palavra-passe antes de ela ser guardada.

Para um serviço que nenhum mosaico cobre, o mosaico **Rclone** em **Configuração própria do rclone** aceita remotes colados de um rclone.conf. São acrescentados aos remotes que o BombVault já tem e nunca os substituem, e ficam guardados encriptados. Para fazer backup de uma secção para um deles, escreve `rclone:<remote>:<pasta>` como repositório externo dessa secção, por exemplo `rclone:nas:backups/bombvault`. A partilha ou o bucket é o primeiro segmento do caminho, não faz parte do nome.

Este é o melhor caminho do que montar a partilha no Unraid: o restic desaconselha manter um repositório numa partilha CIFS montada, e aqui nada é montado. O NFS não tem mosaico porque nem o restic nem o rclone têm um backend NFS. Para NFS, monta a exportação no host e usa-a como pasta local.

### Mover e remover {#move-remove}

- **Mover** leva a cópia de uma secção para outro destino, com todo o seu historial. Tudo fica no lugar antigo. O BombVault não elimina nada aí. A mudança corre em segundo plano e pode demorar: cerca de 9 minutos por 100 GB entre discos locais, horas por uma ligação lenta. A visão geral mostra-a enquanto corre.
- **Remover destino** oferece dois caminhos. **Remover destino, manter os backups** deixa de escrever lá e deixa os backups no lugar. Podes lê-los mais tarde em **Restauração, De uma pasta de backups antiga**. **Remover destino e excluir os backups de lá** lê primeiro o que está guardado lá e lista-o: que backups saem, quais deles não estão em mais lado nenhum, e que repositórios ficam porque a proteção contra eliminação recusa, entradas ainda fazem backup diretamente para eles, outro destino ou secção guarda lá as suas cópias, ou não puderam ser lidos. As entradas que perderiam o seu único backup são listadas à parte e são mantidas, a não ser que ligues **Excluir também estas**. Confirmas escrevendo o nome do destino, e a eliminação corre em segundo plano.
- Isto vale para todos os tipos de destino: um que várias secções usam, um alvo configurado para uma secção, um repositório que configuraste, e a pasta para onde uma secção escreve os seus próprios backups. As entradas que fazem backup para um repositório removido voltam a seguir a sua secção, e o mesmo vale para uma predefinição de secção que apontava para lá. A pasta de uma secção não pode simplesmente sair, porque cada secção precisa de uma. A janela pede a pasta para onde as secções se mudam, e cada secção mantém o nome da sua pasta por baixo dela.
- Eliminar os backups num destino precisa de uma palavra-passe de acesso e de um início de sessão na instância. Sem palavra-passe o destino ainda pode ser removido, só os seus backups não podem ser eliminados daqui.

### Exemplo: uma cópia externa no Backblaze B2 {#example-s3}

Os containers e o auto-backup devem viver também no Backblaze B2, para que um servidor morto não os leve consigo. Os mesmos passos servem para o Wasabi, o Hetzner Object Storage e a maioria dos outros serviços S3.

1. **No B2,** cria um bucket privado, como `tower-backups`, numa região perto de ti. Em **Application Keys**, cria uma chave com acesso de leitura e escrita, limitada a esse bucket, e anota o ID da chave e a chave da aplicação. O B2 mostra a chave da aplicação uma só vez. Define o ciclo de vida do bucket para guardar só a última versão, ou o B2 guarda os dados podados como versões ocultas e cobra-tas.
2. **Fornecedor.** No BombVault, abre **Destinos**, carrega em **Adicionar destino** e escolhe **Backblaze B2** em **Serviço S3**.
3. **Entrar.** Escreve o ID da chave como **ID da chave de acesso** e a chave da aplicação como **Chave de acesso secreta**. Escreve o endpoint do bucket em **Endpoint**, por exemplo `s3.eu-central-003.backblazeb2.com`, e o BombVault lê a região dele. Escreve `tower-backups` em **Bucket**, porque esta chave só abre um bucket.
4. **Testar ligação.** Deve dizer **Ligação estabelecida** e listar as pastas do bucket. Carrega em **Nova pasta** e chama-lhe `bombvault`. O BombVault põe lá dentro uma pasta por secção, como `bombvault/containers`.
5. **Proteção.** A janela explica que o B2 pode guardar versões antigas com bloqueio de objetos e que o BombVault não consegue verificar isso. Deixa **Proteção contra eliminação (append-only)** desligada, a não ser que o bucket recuse mesmo eliminações, para que o BombVault possa podar pelas regras de retenção do destino.
6. **Para que secções.** Marca **Containers** e **Auto-backup**. O auto-backup leva as tuas definições, para que uma instalação nova as possa trazer de volta. A janela diz quantas entradas e, quando se sabe, quanto historial a primeira cópia leva.
7. Carrega em **Salvar destino**.

A primeira cópia corre depois do backup seguinte. Para a iniciar já, abre **Definições, Destinos** e carrega em **Replicar agora** na linha da secção em **Cópias externas**. O cabeçalho de estado diz **Cópia externa em curso**. A primeira cópia leva todo o historial, por isso 200 GB por uma ligação de envio de 100 Mbit/s demoram cerca de quatro horas e meia. As cópias seguintes enviam só o que é novo.

Para manter a tua linha livre, abre a página do destino e define o **Limite de envio (KiB/s)** no cartão **Copiar**, por exemplo `5120`, que se lê como cerca de 5,0 MB/s (41,9 Mbit/s). Já agora, vê **Durante quanto tempo guardar** na mesma página.

Para ver que funcionou, olha para o **Estado de proteção** na visão geral, onde **Externo** passa a **cópia atualizada**, e para o separador **Backups** de um container, onde os seus instantâneos nomeiam agora `tower-backups` como segundo lugar. Depois carrega outra vez em **Descarregar kit de recuperação** em **Definições, Restauro**, porque o kit lista o novo destino e as suas credenciais.

**O Cloudflare R2 funciona da mesma maneira,** com um token de objetos em vez de uma chave. Cola o endereço S3 que o painel do R2 mostra, com o bucket, e o BombVault preenche o servidor e o bucket.

### Exemplo: um rest-server que geres tu {#example-rest-server}

Um rest-server em modo append-only recusa eliminações, por isso uma cópia nele sobrevive mesmo que alguém tome conta do teu servidor. Basta uma segunda máquina, em casa de um amigo ou na garagem. Um BombVault emparelhado pode configurar isto por ti, vê [Servidor recetor](#receiving-server). Para o correr à mão em qualquer anfitrião Docker:

```sh
docker run -d --name rest-server --restart unless-stopped \
  -p 8000:8000 \
  -v /mnt/user/backups/rest-server:/data \
  -e OPTIONS="--append-only --private-repos" \
  restic/rest-server
docker exec -it rest-server create_user bombvault
```

A última linha pede uma palavra-passe e escreve o utilizador no ficheiro `.htpasswd` na pasta de dados.

1. No BombVault, carrega em **Adicionar destino** e escolhe **rest-server** em **NAS ou servidor**.
2. Escreve o endereço com o utilizador como pasta, por exemplo `http://192.168.2.20:8000/bombvault`, e o **Nome de utilizador** e a **Palavra-passe**. Carrega em **Testar ligação**.
3. Em **Proteção**, liga **Proteção contra eliminação (append-only)**. O BombVault nunca poda aí e deixa isso ao outro lado.
4. Marca as secções e carrega em **Salvar destino**.
5. Quando a primeira cópia estiver lá, abre a página do destino e carrega em **Testar agora a proteção contra eliminação**. O BombVault tenta eliminar um objeto de teste. O cartão deve dizer **Último: eliminação recusada**, e a **Proteção contra eliminação** na visão geral passa a **verificada**. Se disser **Último: eliminação não recusada**, falta o `--append-only`.

Como nada poda esta cópia a partir daqui, define o **Orçamento de crescimento (GB, 0 = desativado)** na janela da secção, que se abre com **Editar** na sua linha de **Cópias externas**, para saberes antes de o disco encher. Com um certificado autoassinado, vê a nota sobre `certs` acima.

## A pasta própria de uma secção {#own-folder}

Cada secção tem uma pasta própria: o repositório para onde os seus backups são escritos primeiro. As cópias para outros destinos saem daí. Uma instalação nova mantém as pastas próprias neste servidor, por baixo de `/mnt/user/bombvault`, e juntas formam o destino **Local**.

Uma pasta própria também pode ficar num destino que já tenhas, como um NAS, um rest-server ou um bucket S3. A secção faz então backup diretamente para lá, sem cópia local pelo meio, e a pasta própria segue as credenciais, a proteção contra eliminação e os limites de largura de banda desse destino. O BombVault poda-a depois de cada backup pelo cartão **Durante quanto tempo guardar** do destino, e os valores próprios de uma secção em **Regras por secção** têm prioridade sobre o cartão. O destino lista a secção no seu cartão **Pastas próprias**, e **Destinos por secção** mostra o destino como a casa da secção.

### Mudar uma pasta própria {#move-own-folder}

1. Abre o destino que deve guardar a pasta própria e escolhe **Usar como pasta própria**, ou **Adicionar secções** no seu cartão **Pastas próprias**. Para mudar várias secções de uma vez, escolhe **Mover pastas próprias** ao lado de **Pastas próprias aqui** no cartão **Ligação** do destino Local.
2. Escolhe as secções em **Secções** e o lugar em **Mover para**. **Outro lugar** aceita uma pasta neste anfitrião ou o endereço de um repositório remoto escrito à mão, com umas credenciais das tuas credenciais guardadas.
3. Lê **O que acontece**. Para cada secção diz para onde vão os novos backups, se uma cópia que já está no novo lugar passa a ser a pasta própria, e que backups são copiados primeiro.
4. **Copiar os backups já feitos** vem ligado por predefinição. Desligado, os backups antigos ficam onde estão e ainda se podem restaurar de lá.
5. Escolhe **Mudar**. Quando o cartão **Durante quanto tempo guardar** do novo lugar guarda menos do que a regra atual da secção, o BombVault pergunta primeiro **Guardar menos?** Aí, **Manter os valores atuais** dessa secção **em Regras por secção** leva os valores de hoje, **Guardar mesmo assim** segue em frente e **Voltar** deixa tudo como estava. Quando os backups têm de ser copiados primeiro, o BombVault copia-os e depois muda a pasta própria. Quando já lá estão, verifica-os antes da mudança.

Depois da mudança, os novos backups vão para a pasta própria mudada, também para uma secção que tinha o **Local** desligado e fazia backup diretamente para uma cópia. Uma cópia que já estava no novo lugar passa a ser a pasta própria, e a cópia para lá termina. Onde essa cópia guardava alguns backups mais tempo do que as regras de retenção da secção, a secção guarda o que qualquer das duas regras guarde, para que nenhum se perca.

Quando a mudança não pode avançar, a janela diz porquê numa caixa por baixo de **O que acontece**. Quando só a cópia dos backups antigos está a atrapalhar, desligar **Copiar os backups já feitos** deixa a pasta própria mudar sem eles.

Uma secção precisa sempre de uma pasta própria. Remover o destino que a guarda pede a pasta para onde a secção se muda, vê [Mover e remover](#move-remove). Uma pasta própria por onde os novos backups passam ao lado, porque a secção faz backup diretamente para uma cópia, fica marcada com **(desligada)** onde um destino lista quem a usa.

![A janela que muda uma pasta própria, com O que acontece preenchido para uma secção.](assets/screenshots/storage/move-own-folder-dark.png#only-dark)
![A janela que muda uma pasta própria, com O que acontece preenchido para uma secção.](assets/screenshots/storage/move-own-folder-light.png#only-light)

#### Exemplo: os containers mudam para um NAS {#example-own-folder}

O pool de cache está a encher, e tens um destino chamado `nas`, uma partilha SMB numa segunda máquina. A secção Containers tem 61 backups na sua pasta própria neste servidor e copia para o Backblaze B2.

1. Abre **Destinos**, depois `nas`, e carrega em **Usar como pasta própria**.
2. Marca **Containers** em **Secções**.
3. Lê **O que acontece**. Diz que os novos backups dos Containers vão para a pasta em `nas`, que os 61 backups do Local são copiados para lá primeiro, que as cópias continuam a ir para o Backblaze B2, e que a proteção, os limites de largura de banda e o alarme de crescimento vêm de `nas`.
4. Deixa **Copiar os backups já feitos** ligado e carrega em **Mudar**.

O BombVault copia os backups em segundo plano e depois muda a pasta própria. A página de `nas` lista os Containers em **Pastas próprias**, uma entrada que segue a sua secção nomeia `nas` como a sua casa, e o backup seguinte cai lá, com a cópia para o B2 a sair de `nas`. A partir de agora, o cartão **Durante quanto tempo guardar** de `nas` poda a pasta. A pasta antiga neste servidor fica onde está, e o BombVault não elimina nada aí.

### Escrever um endereço remoto à mão {#remote-primary-repositories}

O cartão **Ligação** do destino Local ainda aceita um endereço remoto para a pasta própria de uma secção: **Alterar** ao lado da secção abre uma janela com um interruptor **Local / Remoto**. **Remoto** aceita um endereço restic como `s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo` ou `rclone:remote:bucket/path`, e as **Definições de segurança do repositório primário remoto** acrescentam um teste de ligação, limites de envio e receção, proteção append-only com o mesmo teste de adulteração que um destino externo recebe, e um alarme de orçamento de crescimento.

Um endereço destes inicia sessão com as credenciais predefinidas. Quando está numa pasta de um destino que tens, a página desse destino lista a secção em **Já está sob este destino**, e assumi-la dá-lhe as credenciais, a classe de armazenamento e a proteção contra eliminação do destino. Mudar a pasta própria para um destino com **Usar como pasta própria** é o caminho mais simples, porque a janela verifica os backups já feitos antes de mudar o que quer que seja.

## Localização por entrada {#placement}

O cartão **Agendamento e destino** no separador Definições de uma entrada tem uma fila de botões: **Local** e um botão por cada destino externo da secção. Os botões acesos recebem os backups da entrada.

- Com **Local** aceso, a entrada é escrita no repositório mostrado em **Guardado em** e copiada para todos os outros destinos acesos. Apaga um destino e ele não recebe nada de novo desta entrada. Só o Local não copia para lado nenhum, o que serve para dados que já têm uma segunda cópia, por exemplo uma partilha que vive num NAS.
- Com **Local** apagado, a entrada é escrita diretamente no repositório direto do primeiro destino aceso e copiada daí para os outros acesos. Da primeira vez, uma janela cria esse repositório direto.
- Um botão fica sempre aceso, porque um backup precisa de um lugar para onde ir. Para deixar uma entrada de fora dos backups, usa **Não fazer backup**.

O BombVault nunca move backups entre repositórios. Uma entrada que segue a sua secção acompanha-a quando a predefinição da secção muda: o seu backup seguinte cai no novo lugar, e os backups que tem ficam onde estão. Um destino escolhido na própria entrada fica fixo desde o seu primeiro backup. O cartão **Repositório** no separador Definições nomeia-o. As cópias podem mudar a qualquer altura. Um destino que já não recebe uma entrada mantém as cópias que tem e apara-as pela sua própria retenção na próxima execução externa da secção. Nos destinos append-only não se pode eliminar.

O lugar que uma entrada deixou mantém os seus backups. No separador **Backups** da entrada e na **Restauração** aparece como **Só cópias anteriores**, uma restauração ainda pode ler dele, e uma poda aí deixa esses backups em paz.

**Seguir a predefinição** devolve à sua secção uma entrada com uma escolha própria. Pergunta primeiro, diz para onde vão então os novos backups, e nomeia o que cada destino deixa de receber e o que a execução seguinte enviaria.

Por baixo da fila, o cartão diz para onde vai a entrada e o que lá está de facto: quantos locais a guardam, quando cada destino foi visto pela última vez, e se o 3-2-1 está cumprido. Um local é o servidor com os dados originais, cada destino externo e cada repositório marcado **Fora das instalações**. O BombVault verifica cópias e locais. Não verifica a parte dos "dois suportes" do 3-2-1.

### Predefinições por secção {#placement-defaults}

O cartão **Destinos por secção** na página Destinos desenha a predefinição de cada secção como um fluxo 3-2-1. Cada linha de uma secção para um destino é um interruptor, o **Local** incluído, para todas as secções, também a unidade flash, os conjuntos de dados ZFS e o auto-backup. Um clique numa linha ou num ponto liga-a ou desliga-a logo, e as linhas tracejadas estão desligadas. Só desligar o **Local** de uma secção pergunta primeiro, na mesma janela do cartão de predefinição, porque a secção passa então a fazer backup diretamente para uma cópia. **Cancelar** deixa tudo como estava. **Gráfico / Interruptores** mostra o mesmo em filas de botões. Um interruptor decide para onde vão os novos backups. As entradas sem escolha própria, e as pastas de projeto das stacks Compose, acompanham-no logo, por isso o seu backup seguinte vai para os novos lugares. Os backups já guardados ficam onde estão e ainda se podem restaurar de lá. O flash, os conjuntos de dados ZFS e o auto-backup fazem backup para a sua pasta local e copiam para fora a partir desse backup. A sua linha para o **Local** também pode ser desligada. Os seus backups vão então diretamente para os seus destinos externos, e uma entrada ZFS com uma réplica e nenhum destino só envia a sua réplica.

Com o **Local** desligado e só destinos externos ligados, uma secção faz backup diretamente para eles, e o BombVault escolhe aí o repositório por si. Os containers, as VMs e os conjuntos de pastas vão para o repositório direto do primeiro destino ligado, descrito abaixo, e são copiados daí para os outros. O flash, os conjuntos de dados ZFS e o auto-backup escrevem cada backup em todos os destinos ligados, e cada destino guarda-os pela sua própria retenção.

Uma secção com todas as linhas desligadas diz **Sem backup**. As suas entradas não recebem backups e contam como não protegidas.

As entradas que escolheram na sua página os seus próprios destinos mantêm-nos. O fluxo diz quantas há numa secção, por exemplo **2 entradas diferem**, e leva até elas: ao separador Definições da entrada quando há uma, a uma lista com o sítio para onde cada uma faz backup quando há várias. **Seguir a predefinição** aí devolve uma entrada à secção.

Um novo destino externo recebe todas as entradas que não estão definidas para Local. A janela que o junta diz quantas entradas e, quando se sabe, quanto historial isso é.

### Repositórios diretos {#direct-repositories}

Um repositório direto é onde as entradas fazem backup diretamente para um destino externo. O BombVault põe-no ao lado da pasta do destino com `-direct` acrescentado ao nome, ou, quando o destino está na raiz do seu bucket, do seu remote ou de um utilizador do rest-server, numa pasta lá dentro com o nome da secção, como `containers-direct`. No fluxo da página Destinos é criado sem perguntar. Desligar o Local para uma só entrada, de modo que um destino sem repositório direto passe a ser a sua casa, abre uma janela com a localização sugerida, por exemplo `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, e um teste de ligação que não cria nada. **Criar e usar** cria o repositório e aponta a entrada para ele. Um repositório direto não tem definições próprias. Segue o interruptor, as credenciais, a classe de armazenamento, os limites, a definição append-only e a retenção do seu destino, e muda com eles. A página do destino lista-o em **Guarda diretamente aqui**, com cada secção que faz backup diretamente para ele e quantas entradas o fazem. Um para onde já nada faz backup di-lo aí e pode ser removido, e os backups no seu lugar ficam. Quando uma chave nova do destino não o consegue abrir, o repositório direto mantém a chave que tem e a gravação di-lo. Uma entrada num repositório direto é copiada daí para os outros destinos ligados, nunca para o destino a que o repositório pertence. Os seus instantâneos levam a etiqueta `bv:direct`, e todas as outras passagens de retenção mantêm-nos, por isso um repositório direto que perdeu a ligação ao seu destino nunca envelhece pelas regras locais. O B2 é alcançado pelo seu endpoint S3, com o ID da chave e a chave da aplicação introduzidos como chaves S3. Uma chave limitada à pasta do próprio destino não consegue alcançar a pasta ao lado, por isso limita a chave à pasta acima.

### Fora das instalações {#off-the-premises}

Um destino ou um repositório pode ser marcado como **Fora das instalações**. Os remotos começam marcados. Desliga-o para um rest-server no mesmo edifício. A marca só conta locais e o 3-2-1 nos cartões. Não muda nenhuma cópia.

Uma réplica ZFS é uma cópia fora das instalações, mas não um backup com historial. Guarda só os instantâneos que definires, e as versões antigas e os ficheiros individuais continuam a vir dos backups. Vai de um servidor ZFS diretamente para o outro, por SSH para um servidor ZFS ou para o endpoint de receção de uma instância emparelhada. O grupo de emparelhamento só transporta o pedido. Vê [Réplica](zfs-datasets.md#replica).

### Depois de uma reconstrução {#after-a-rebuild}

As escolhas de cópia vivem nas definições do próprio BombVault. Depois de uma reconstrução por **Descobrir backups** ou pelo assistente de emergência sem definições restauradas, desaparecem, e copiar tudo mandaria outra vez para o B2 as entradas que tinhas deixado de fora. As cópias externas de todas as secções reconstruídas ficam por isso em pausa. A visão geral di-lo, e o cartão **Destinos por secção** oferece **Confirmar padrão** com uma pré-visualização do que a próxima execução copia e os nomes nos backups que não têm entrada, que podes deixar de fora aí. Só a confirmação termina a pausa. Importar um ficheiro de definições traz de volta regras e predefinições, mas não a termina.

## Proteção contra eliminação (append-only) {#delete-protection-append-only}

Liga a proteção contra eliminação num destino externo para que o ransomware, ou um host comprometido, não possa eliminar nem reescrever os teus backups. O outro lado (um `restic/rest-server` a correr em modo `--append-only`) **impõe-na**. O BombVault só a **verifica** e nunca mostra verde com base apenas numa afirmação de configuração. Uma pasta neste servidor não consegue recusar a eliminação, porque quem controla o BombVault controla também a pasta, por isso os destinos locais não oferecem o interruptor.

O interruptor pertence ao destino: uma secção que copia para lá recebe-o do destino, por isso liga-o a partir da página do destino em **Destinos**. A **Verificar proteção contra eliminação** agendada só corre enquanto uma secção copia para um destino com o interruptor ligado. O outro lado tem de ser iniciado com `--append-only`. Com a imagem `restic/rest-server`, no TrueNAS como em qualquer outro anfitrião Docker, isso vai na variável `OPTIONS`.

Alguns lugares não conseguem recusar uma eliminação de todo, um disco na nuvem como o OneDrive, por exemplo. Se manténs lá cópias de propósito, liga **Aceitar sem proteção contra eliminação** na página do destino. A visão geral mostra então a proteção contra eliminação em falta de uma secção que só copia para destinos aceites como um risco aceite, um anel cinzento em vez de um ponto âmbar, e a cor da secção segue as suas outras verificações. Mais nada muda: os backups lá ainda podem ser eliminados, o BombVault ainda os poda pelas regras de retenção do destino, e o interruptor desaparece quando a proteção contra eliminação é ligada. Uma cópia que passa para outro lugar perde a aceitação. Ligar a proteção contra eliminação também a limpa.

O caminho mais rápido para uma cópia append-only é um BombVault emparelhado com um [servidor recetor](#receiving-server): corre o rest-server por ti, em modo append-only.

!!! note "Uma eliminação bem-sucedida em `/locks/` é esperada"
    Append-only não significa que nada possa alguma vez ser removido. O restic tem de criar e libertar os seus próprios bloqueios, por isso `/locks/` continua gravável e eliminável por conceção. Os instantâneos e os dados por trás deles, que é o que o ransomware procuraria, não podem ser removidos. Se sondares tu o outro lado, uma eliminação que tem êxito em `/locks/` é o comportamento correto e não um buraco na proteção.

!!! warning "Os repositórios append-only nunca são podados a partir desta máquina"
    Um destino append-only nunca poda instantâneos antigos a partir daqui. Podar é tarefa do outro lado. Define um **alarme de orçamento de crescimento** para ele, para seres avisado antes de o tamanho do repositório disparar.

## Teste de adulteração {#tamper-test}

O BombVault prova periodicamente a garantia append-only ao tentar de facto uma eliminação contra o repositório externo, dirigida a um objeto que não existe. **Testar agora a proteção contra eliminação** na página de um destino corre-o a pedido, e **Verificar proteção contra eliminação** em **Definições, Verificação, Verificações automáticas** corre-o num agendamento.

Desligar **Verificar proteção contra eliminação** não esconde uma lacuna. Um destino com proteção contra eliminação que nunca foi testado lê-se então "não provada, a sua verificação está desligada", e um destino sem proteção contra eliminação lê-se "nenhuma" de qualquer forma.

Uma cópia que está desligada não é testada, e o destino di-lo. O teste de ligação continua a correr nela, porque só lê, por isso uma cópia pode ser verificada antes de ser ligada.

- **Recusada** significa protegida.
- **Aceite** significa não protegida.
- Um resultado **inconclusivo** (servidor inalcançável, erro de autenticação) nunca inverte o veredicto guardado.

Uma inversão real de protegido para não protegido dispara um único alerta.

## Testes DR {#dr-drills}

O BombVault oferece dois níveis de prova de que os teus backups são mesmo restauráveis, e não apenas presentes.

- **Testes de verificação de restauro (locais).** O BombVault corre periodicamente `restic check --read-data-subset` (limitado, nunca uma restauração completa que encha o disco). A cadência fica em **Definições, Verificação, Verificações automáticas**, e o cartão **Verificar os backups** no mesmo mosaico mostra o resultado.
- **Restauro de teste (externo).** O BombVault restaura um alvo real do repositório externo para uma sandbox selada, verifica-o ficheiro a ficheiro e byte a byte, e depois limpa. Isto prova que consegues recuperar a partir do externo, e não apenas que o repositório responde.

O cartão **Estado de proteção** na visão geral resume isto por secção em quatro verificações: um backup atual neste servidor, uma cópia externa, a proteção contra eliminação lá e um restauro de teste a partir de lá. Um clique numa secção mostra os detalhes, e o cartão só fica verde com factos verificados. Um destino que aceitaste sem proteção contra eliminação conta aí como um risco aceite, não como um aviso.

## Emparelhar instâncias {#pairing}

Vários BombVault trabalham juntos como um grupo: mostram o estado de proteção uns dos outros, assumem funções uns para os outros e podem guardar a chave uns dos outros. Uma instância junta-se ao grupo com doze palavras.

![A página de emparelhamento com os seus três cartões de passos.](assets/screenshots/offsite/pairing-dark.png#only-dark)
![A página de emparelhamento com os seus três cartões de passos.](assets/screenshots/offsite/pairing-light.png#only-light)

Na primeira instância abre **Definições, Emparelhamento** e carrega em **Gerar frase**. Três cartões de passos mostram como se faz. Doze palavras aparecem numa janela com um botão **Copiar**. Em cada uma das outras instâncias abre o mesmo sítio, carrega em **Introduzir frase** e cola ou escreve-as, ou carrega em **Colar** nessa janela. Uma palavra que não está na lista é nomeada com a sua posição enquanto escreves, e a última palavra leva uma soma de verificação, por isso uma palavra mal escrita ou trocada é apanhada antes de qualquer emparelhamento. Gera a frase numa só instância: duas instâncias que gerem uma cada formam dois grupos separados. Se ninguém aparecer durante um minuto, a página oferece duas saídas: mostrar as palavras outra vez para as introduzir do outro lado, ou introduzir as palavras da outra instância e juntar-se ao seu grupo num só passo. O emparelhamento funciona sem palavra-passe de acesso, mas define uma: sem ela, qualquer pessoa que consiga abrir a interface web pode ler as palavras e, através do grupo, pedir funções que entregam a palavra-passe do restic das instâncias nele. O cartão de emparelhamento di-lo até haver uma palavra-passe. Com palavra-passe, mostrar a frase outra vez pede-a. **Sair do grupo** tira uma instância outra vez. O cartão **Instâncias** na página de emparelhamento lista os membros do grupo. A imagem de quem faz o quê por quem está na página Instâncias.

Quem souber as palavras pode juntar-se ao grupo, por isso trata-as como uma palavra-passe.

**Como os membros se alcançam.** Cada instância aprende o seu próprio endereço na rede pelo teu navegador no momento em que inicias sessão, listado em **Endereços conhecidos** no cartão **Esta instância** na página de emparelhamento, juntamente com os que a instância encontrou sozinha. Corrige-o aí se um proxy inverso ou uma porta invulgar estiver pela frente. Na mesma rede, os membros anunciam esse endereço por multicast e falam diretamente, e onde o multicast não consegue atravessar uma rede de containers, como a bridge predefinida do Docker, uma instância procura antes na sua própria sub-rede pelas outras com uma chamada assinada a que só um membro do grupo pode responder, de modo que o emparelhamento acaba na mesma em segundos sem relay. Se nada aparecer, **Não consegues encontrá-la?** aceita um endereço escrito à mão, para outra sub-rede ou uma porta fora do normal. As instâncias em redes diferentes passam por um relay, escolhido numa fila de quatro no cartão **Relay** da mesma página. Cada forma vem com uma imagem:

- **Relay do projeto** (a predefinição): `parleyport.halleluja.design`, o relay que o KnightLoader também usa. Nada a configurar.
- **Relay próprio**: o container [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) das Community Apps do Unraid (também em **Definições, Aplicações**). Escreve o seu endereço em cada instância que o deva usar.
- **Instância BombVault**: uma das tuas instâncias que já é alcançável do exterior, com **Servir como relay** ligado. Essa instância responde então em `/relay/connect` no seu próprio endereço, atrás do proxy inverso e do certificado que já tem, e deixa entrar só o teu grupo. Escreve o seu endereço em cada instância que o deva usar.
- **Sem relay**: os membros encontram-se automaticamente na mesma rede, e em mais lado nenhum.

**O que o relay vê.** Um botão com o mesmo nome ao lado das quatro formas mostra isto para a forma que escolheste. Cada chamada entre membros é selada com AES-256-GCM sob uma chave derivada das doze palavras, e essa chave nunca sai das tuas instâncias. O relay fica a saber um hash que agrupa as ligações, para que instância é uma mensagem, que tamanho tem e quando passa. Uma chamada direta na rede local é selada da mesma maneira e assinada também, por isso nada depende do certificado autoassinado que uma instância serve. As chamadas que entregam algo (respostas a funções, uma palavra-passe do restic, inícios de sessão de recetores, depósitos de chaves) são também assinadas com uma chave do membro que as envia, fixada à primeira vista. Vê [Modelo de segurança](configuration.md#security-model).

**O que viaja pelo grupo.** O estado de proteção na página Instâncias, um pedido para verificar já uma secção, os pedidos de função e as suas respostas, as ofertas de armazenamento, e o que um recetor ou recolhedor precisa depois de a sua função ser permitida: as localizações dos repositórios da outra instância e a sua palavra-passe do restic. Os dados de backup nunca. Esses continuam a ir diretamente para os backends do restic. A APP_KEY também não: a palavra-passe do restic abre os repositórios dessa instância e mais nada, nem os seus segredos guardados, sessões ou códigos de recuperação.

**Entradas de antes do emparelhamento.** As instâncias adicionadas com um token de frota, e os repositórios recebidos e as origens de recolha configurados com a APP_KEY da outra instância, ficam depois da atualização e são marcados **Emparelhar novamente**. Continuam a funcionar: no seu primeiro arranque o BombVault substitui cada APP_KEY guardada pela palavra-passe do restic derivada dela. Emparelha as duas instâncias, depois edita a entrada e escolhe a sua instância. Até lá, essas entradas ficam no cartão **Sem instância** na página Instâncias.

O único sítio que ainda aceita uma APP_KEY à mão é [De uma pasta de backups antiga](#restore-from-another-bombvault-repo), para o caso de a outra instância ter desaparecido e não poder responder num grupo.

## Instâncias e funções {#roles}

A página **Instâncias** (um mosaico de definições que pode ser fixado na barra lateral) mostra o grupo como uma rede de quem faz o quê por quem, um cartão para esta instância e um cartão para cada outro membro, os telemóveis Android e os servidores ZFS. **Emparelhamento**, em baixo à direita, leva à página de emparelhamento. Que partes estão ligadas define-se em **Definições, Instâncias, Vários servidores**: **Instâncias**, **Recetor** e **Recolha**.

![A página Instâncias: o grupo como uma rede, e um cartão por instância com os seus mosaicos de função, um deles Inativa.](assets/screenshots/offsite/instances-dark.png#only-dark)
![A página Instâncias: o grupo como uma rede, e um cartão por instância com os seus mosaicos de função, um deles Inativa.](assets/screenshots/offsite/instances-light.png#only-light)

Cada cartão mostra o estado de proteção do membro e as suas funções como mosaicos. Uma função é algo que uma instância faz por outra:

- **Recetor** guarda as cópias externas deste servidor num rest-server em modo append-only. Este servidor envia depois de cada backup e não pode eliminar nada lá, mesmo que alguém tome conta dele.
- **Recolhedor** vai buscar os backups deste servidor no seu próprio agendamento. Este servidor não tem de estar acordado para isso.
- **Servidor ZFS** recebe as réplicas ZFS deste servidor. Cada entrada ZFS escolhe-o em **Réplica** nas suas definições.
- **Guardião da chave** guarda uma cópia selada do kit de recuperação deste servidor. Vê [Guardião da chave](#key-keeper).

Ligar uma função envia um pedido, e nada começa até alguém na outra instância carregar em **Permitir**. O pedido aparece no cartão dessa instância como um selo, e os seus **Detalhes** listam-no com **Permitir** e **Recusar**. Uma função permitida pode ser revogada aí a qualquer altura. Uma linha tracejada na rede é uma função ainda à espera. Uma instância no BombVault 9.x não consegue responder a um pedido, por isso uma função fica aí ativa assim que a ligas, e um pedido de uma instância 9.x espera aqui por **Permitir**.

Os **Detalhes** num cartão abrem o resto: os repositórios recebidos e as origens de recolha desse membro, ofertas de armazenamento, monitorização, **Remover** e **Esquecer a chave**.

Um membro pode oferecer o seu próprio armazenamento externo a outro membro pelo grupo. O outro administrador vê a oferta nos **Detalhes** do membro e aceita-a ou recusa-a. Aceitar cria umas credenciais guardadas e um destino normal, depois das mesmas verificações de um destino que escreves tu. Só viajam assim os dados de ligação, nunca dados de backup. **Verificar agora** pede a um membro que verifique o repositório de uma secção. Nada na página pode iniciar um backup, uma restauração ou eliminar o que quer que seja noutra máquina.

As instâncias que estão desligadas e os telemóveis com a aplicação fechada mantêm o seu cartão como **Inativa**, em cinzento, com quando foram vistos pela última vez, e ficam esbatidos na rede até os **Remover**es. Logo depois de este servidor arrancar, um cartão assim diz **A verificar** até o grupo voltar. **Remover** funciona num cartão destes sejam quais forem as partes ligadas, e termina também os pedidos em aberto do membro, os seus espaços de receção ZFS inativos e os seus pedidos de chave à espera. O cartão **Instâncias** na página de emparelhamento lista também esses membros como **Inativa**.

## Recetor e recolhedor {#receiver}

Na máquina que **recebe** cópias append-only de outro BombVault, os repositórios recebidos dão-te monitorização independente e só de leitura no hardware recetor, para que uma falha silenciosa do outro lado não passe despercebida. Liga o **Recetor** em **Definições, Instâncias**, depois abre os **Detalhes** da instância que envia na página Instâncias e junta o repositório recebido (só de leitura, aberto com a palavra-passe do restic da instância que envia, que ele recebe pelo [grupo de emparelhamento](#pairing)). Recebes:

- **Um inventário por origem**, para veres exatamente que containers, VMs e conjuntos de pastas chegaram.
- **Última receção** por origem, para saberes quão recente é cada uma.
- **Um `restic check` independente** executado no hardware recetor, para que a integridade seja verificada onde os dados estão de facto, e não só no emissor.
- **Um interruptor de homem morto:** um alerta quando uma origem deixa de enviar dentro de uma janela que definires.
- **Alertas de integridade:** um alerta quando uma verificação do lado recetor falha.

O recetor é estritamente só de leitura. Nunca escreve no repositório recebido, por isso nunca pode quebrar a garantia append-only de que o emissor depende.

Um recolhedor funciona ao contrário: com a **Recolha** ligada, uma instância lê os backups de outro membro no seu próprio agendamento, para um repositório próprio por cada origem, por isso nunca contam como backups desta máquina. As suas origens ficam também nos detalhes desse membro: a localização do repositório, que tipo de backup guarda e com que frequência recolher. A palavra-passe do restic vem pelo grupo, nunca a APP_KEY, e o outro lado não tem mais nada para configurar e não tem de estar a correr. A origem é apenas lida, e cada lado mantém as suas próprias credenciais. Uma origem `rclone:` é recusada, porque o rclone a alcançaria com os remotes desta instância.

### Servidor recetor {#receiving-server}

A máquina recetora pode também correr o rest-server para onde os outros copiam. É configurado na primeira vez que permites um pedido de **Recetor** que o pede: a janela diz que o servidor precisa de um servidor recetor e pede uma pasta numa partilha, com **Nova pasta** para criar uma, e uma porta (8000, a menos que outro container a use). O BombVault então:

1. recusa se já existir um container chamado `rest-server` ou se outro container tiver a porta,
2. transfere o `restic/rest-server` e arranca-o pelo socket Docker em modo append-only com repositórios privados e um ficheiro de logins na pasta,
3. escreve o seu template Unraid na unidade flash, para o container continuar editável no separador Docker, ou oferece o template como transferência quando a flash está fora de alcance,
4. corre o teste de adulteração contra ele e mostra se recusa eliminações.

As instâncias do teu grupo encontram então o servidor entre os fornecedores em **Do teu grupo**, com o nome da máquina recetora, e a função **Recetor** oferece-o como a escolha segura. Cada instância recebe um início de sessão próprio na primeira vez que usa o servidor, e escreve só na sua própria pasta aí. Com o **Recetor** ligado em **Definições, Instâncias**, **Servidor recetor** no cartão desta instância abre o servidor outra vez. A sua janela lista os inícios de sessão em **Parceiros com início de sessão**, e **Revogar início de sessão** tira um. O que essa instância já copiou fica na pasta. A configuração cria também um **Início de sessão fora do grupo**, cuja palavra-passe a janela mostra uma só vez.

Uma instância que só alcança a máquina recetora pelo relay não pode usar o servidor, porque o relay não transporta backups. Junta primeiro o endereço da máquina recetora na página de emparelhamento. Quando o BombVault corre num endereço IP próprio (em br0, por exemplo), preenche **Endereço para parceiros**, porque o servidor escuta no endereço do host.

## Exemplo: emparelhar duas caixas Unraid {#worked-example-two-unraid-boxes-end-to-end}

Esta é uma configuração completa das peças acima, porque é mais fácil montá-las depois de as ter visto montadas.

Duas máquinas: a **TOWER** corre os containers e envia backups. A **VAULT** recebe-os e impõe o append-only. Substitui pelos teus próprios nomes, endereços e caminhos de partilha.

**1. Emparelha as duas máquinas.** Na TOWER abre **Definições, Emparelhamento**, carrega em **Gerar frase**, e escreve as doze palavras na VAULT em **Introduzir frase**. Define primeiro uma palavra-passe de acesso nas duas.

**2. Na TOWER, pede à VAULT que seja o recetor.** Na página Instâncias, liga o **Recetor** no cartão da VAULT, escolhe as secções e o **servidor recetor da VAULT** como lugar, e envia o pedido.

**3. Na VAULT, permite-o.** A página Instâncias da VAULT mostra o pedido no cartão da TOWER. Carrega em **Permitir**. Da primeira vez, a VAULT pede uma pasta para o seu servidor recetor, por exemplo `user/appdata/rest-server`, arranca o rest-server em modo append-only e corre o teste de adulteração contra ele.

**4. De volta à TOWER.** A partir daí a TOWER copia para a VAULT depois de cada backup, e o novo destino aparece na página Destinos da TOWER com a sua proteção contra eliminação.

O que responde o teste de adulteração:

| Resultado | O que aconteceu |
| --- | --- |
| **protegida** | A VAULT recusou a eliminação. É o único estado que passa. |
| **não protegida** | A VAULT aceitou uma eliminação. Falta o `--append-only` ou foi removido. |
| **inconclusivo** | Nenhum dos dois. Normalmente o URL não é o que o próprio restic usa, ou as credenciais mudaram. Nada é registado e nenhum alerta dispara. |

**5. Vê o que chega, se quiseres.** Liga o **Recetor** em **Definições, Instâncias** na VAULT, abre os **Detalhes** da TOWER e junta o repositório recebido. A VAULT recebe a palavra-passe do restic da TOWER pelo grupo. Ninguém escreve uma chave.

**6. Torna-o mútuo.** Repete os passos no outro sentido: a VAULT pede à TOWER que seja o seu recetor. Cada máquina impõe então o append-only à outra, e nenhuma pode eliminar os backups da outra.

Um rest-server que geres à mão também serve: junta-o com **Adicionar destino** como rest-server, com o utilizador e a palavra-passe do seu `.htpasswd`, e liga a proteção contra eliminação. Mantém o `--append-only` nas suas opções. Sem ele, o outro lado volta a ser uma partilha normal.

!!! warning "Um caminho local é um caminho dentro do container"
    Onde o BombVault pede uma pasta neste servidor, escreve-a relativa à montagem do host, por exemplo `user/appdata/rest-server/bombvault-containers`, não `/mnt/user/appdata/...`. O BombVault corre num container, onde o `/mnt` do host está montado noutro sítio. Um caminho absoluto do host não existe lá dentro. Se colares um, o BombVault diz-te o caminho relativo a usar.

## Assistente de emergência {#emergency}

O **Assistente de emergência** traz de volta um servidor que desapareceu ou foi configurado de novo. Começa na página **Restauração** em **Emergência**, e a partir de **Restaurar a partir de um backup** na página de boas-vindas de uma instalação nova. Corre em quatro passos: **Acesso**, **Definições**, **Seleção** e **Restauração**.

1. **Acesso.** Mostra ao BombVault a chave dos backups antigos, de uma de três maneiras:
    - **Carregar kit de recuperação**: o ficheiro que descarregaste durante a configuração. Contém a chave e todos os destinos, e o BombVault preenche o resto a partir dele. Um kit selado com age pede a identidade para a qual foi selado. É usado uma só vez e não fica guardado.
    - **Através do grupo**: uma instância do teu grupo guarda a chave como depósito. Precisas das doze palavras do grupo, do teu código de desbloqueio e de alguém nessa instância. Vê [Guardião da chave](#key-keeper).
    - **Escrever a chave**: a `APP_KEY` do servidor antigo, e onde estão os seus backups.

    O BombVault experimenta então a chave em todos os destinos e diz para cada um se está legível, não encontrado, trancado com outra chave, inalcançável ou ainda sem credenciais. Nada foi guardado ainda, e a chave fica na memória por um tempo limitado. **Adotar a chave** faz este servidor escrever os seus backups com essa chave daqui em diante. **Anular a adoção** volta à sua própria chave até o primeiro backup ter corrido. Um servidor que já tem backups próprios recusa a adoção e aponta para [De uma pasta de backups antiga](#restore-from-another-bombvault-repo). **Guardar credenciais** grava as credenciais que o kit traz para os destinos remotos, e pergunta antes de substituir credenciais que este servidor já tem.
2. **Definições.** **Trazer de volta as definições** restaura as definições do próprio BombVault a partir do auto-backup mais recente: destinos, credenciais, agendamentos e regras. O BombVault reinicia-se para as aplicar, pelo socket Docker, para que a base de dados de definições em uso nunca seja sobrescrita com um acesso aberto, e o assistente continua sozinho depois. O início de sessão deste servidor fica: continuas a entrar com a palavra-passe que usas agora. Os segredos que o servidor antigo tinha guardados e que não podem ser adotados são listados, para os poderes introduzir outra vez. A restauração termina todas as sessões e remove as chaves de acesso, por isso configura-as de novo depois.
3. **Seleção.** O BombVault percorre os teus backups e lista os containers, VMs, conjuntos de pastas e conjuntos de dados ZFS que encontra. O instantâneo mais recente vem pré-selecionado em todo o lado. O que este servidor ainda tem começa desligado, porque uma restauração substituí-lo-ia. Os conjuntos de pastas já não sabem de onde vieram depois de uma reconstrução, por isso vão para uma pasta que escolhes.
4. **Restauração.** Tudo o que foi selecionado volta de uma só vez, deixado parado se quiseres arrancar tu cada container e VM, o que é o mais seguro depois de uma perda total. Depois descarrega um kit de recuperação novo: o antigo pertence à instalação anterior.

![O assistente de emergência no primeiro passo, com as três formas de entrar.](assets/screenshots/offsite/emergency-dark.png#only-dark)
![O assistente de emergência no primeiro passo, com as três formas de entrar.](assets/screenshots/offsite/emergency-light.png#only-light)

### Exemplo: um servidor novo a partir do kit de recuperação {#example-emergency}

O servidor TOWER morreu com os seus discos. Tinha 23 containers e 2 VMs, e copiou-os, com o seu auto-backup, para o Backblaze B2. Tens o `bombvault-recovery-kit.md` do teu gestor de palavras-passe e uma máquina nova com Unraid. A sua antiga `APP_KEY` perdeu-se, e não precisas dela.

1. **Instala o BombVault** na máquina nova e dá-lhe uma `APP_KEY` nova, como em qualquer instalação nova. Configura a ligação das VMs em **Definições, Integrações, SSH do anfitrião** se quiseres as VMs de volta, porque uma restauração de VM precisa dela.
2. **Inicia o assistente.** Em **Damos-te as boas-vindas ao BombVault**, escolhe **Restaurar a partir de um backup**. Na página Restauração, escolhe **Iniciar o assistente de emergência** em **Emergência**.
3. **Acesso.** Escolhe **Carregar kit de recuperação** e larga o ficheiro. O BombVault experimenta a sua chave em todos os destinos e diz, por exemplo, **Kit de recuperação lido. A sua chave abre 0 de 2 destinos.** A pasta local diz **não encontrado**, já que o disco morreu, e o Backblaze B2 diz **faltam credenciais**.
4. **Guardar credenciais.** O kit traz as credenciais dos seus destinos remotos. Carrega em **Guardar credenciais**, e os destinos são experimentados de novo, por isso o Backblaze B2 diz agora **legível**.
5. **Adotar a chave.** Este servidor escreve agora os seus backups com a chave antiga, para que os novos backups encaixem nos antigos. Mantém a sua própria `APP_KEY` no template. **Anular a adoção** funciona até o primeiro backup ter corrido.
6. **Definições.** Escolhe **Trazer de volta as definições**, que lê o auto-backup mais recente do B2. O BombVault reinicia-se para o aplicar, e o assistente continua sozinho. Os teus destinos, agendamentos e regras estão de volta, e continuas a entrar com a palavra-passe deste servidor. Escreve outra vez os segredos que o assistente lista, como as definições de notificações.
7. **Seleção.** Põe **Onde estão os backups?** nas cópias externas, porque as locais foram abaixo com o servidor antigo. O BombVault lista os 23 containers e as 2 VMs com o instantâneo mais recente pré-selecionado, e podes escolher outro para qualquer um.
8. **Restauração.** Liga **Deixar parado depois**, o que é o mais seguro depois de uma perda total, e começa. **O que acontece** diz o que volta. A linha de resultado lê-se, por exemplo, **25 restaurados, 0 falhados. Inicia-os em Backups quando for preciso.**
9. **Terminar.** Arranca primeiro as tuas bases de dados, depois as aplicações que as usam. Abre **Definições, Restauro** e descarrega um kit de recuperação novo, porque o antigo pertence à instalação anterior. Vê **Precisa de atenção** na visão geral, e corre **Fazer backup de tudo agora** quando tudo funcionar.

Se o auto-backup também faltar, escolhe **Ignorar, não tenho Auto-backup**. Os destinos vêm então só do kit, configuras os agendamentos e as regras de novo, e as cópias externas ficam em pausa até confirmares as predefinições das secções, vê [Depois de uma reconstrução](#after-a-rebuild).

!!! tip "Migração planeada ou desastre"
    O assistente de emergência restaura as definições do próprio BombVault a partir de um backup. Para uma mudança *planeada* para uma máquina nova, podes antes levar a tua configuração diretamente com o cartão **Exportar / importar configurações** (um ficheiro JSON portátil). Vê [Configuração](configuration.md#portable-settings-export-and-import).

### De uma pasta de backups antiga {#restore-from-another-bombvault-repo}

**De uma pasta de backups antiga**, em **Outras origens** na página Restauração, abre o repositório de uma instância BombVault *diferente* (uma partilha montada em `/mnt`, ou um endereço remoto) com o kit de recuperação dessa instância ou a sua `APP_KEY` escrita à mão, numa sessão única e só de leitura. Percorre os containers, VMs, conjuntos de pastas e conjuntos de dados ZFS guardados lá, escolhe um instantâneo e restaura-o, e o objeto restaurado passa a ser um container, VM ou conjunto de pastas local normal. Nunca se escreve nada no outro repositório, e as tuas definições de backup ficam intactas (a sessão vive na memória e expira sozinha). Mudar um container do servidor A para o servidor B não obriga a reapontar as tuas definições e a revertê-las depois. Esta janela é de uma só vez: abre uma sessão, restaura o que escolheres e esquece a outra instância. Se queres antes uma disposição permanente, em que esta máquina vai buscar os instantâneos de outra instância para o seu próprio repositório num agendamento, essa é a função **Recolhedor** na página Instâncias.

Um container cuja rede não existe neste servidor, como uma rede `br0` do Unraid num anfitrião Docker simples, mostra um seletor de rede por baixo da sua linha. O BombVault cria-o na rede que escolheres, juntamente com as suas outras redes. O IP fixo e o endereço MAC pertenciam à rede antiga e são deixados cair, por isso a nova rede atribui-os.

## Guardião da chave {#key-keeper}

Um guardião da chave é uma instância do teu grupo que guarda uma cópia selada do kit de recuperação deste servidor. Não consegue abrir a cópia sozinho. Se este servidor alguma vez se perder, uma instalação nova que se junte ao teu grupo pede a cópia ao guardião e abre-a com o teu código de desbloqueio.

![A janela do código de desbloqueio: vinte caracteres em quatro grupos, mostrados uma só vez.](assets/screenshots/offsite/recovery-code-dark.png#only-dark)
![A janela do código de desbloqueio: vinte caracteres em quatro grupos, mostrados uma só vez.](assets/screenshots/offsite/recovery-code-light.png#only-light)

1. Na página Instâncias, liga o **Guardião da chave** no cartão da instância que o deve guardar, e escolhe **Fazer depósito**. Fazer um depósito precisa de uma palavra-passe de acesso.
2. Este servidor mostra o **Código de desbloqueio** uma só vez: vinte letras e dígitos em quatro grupos de cinco. Anota-o e confirma com **Anotei o código**. Este servidor não guarda o código, só o que precisa para selar um depósito para ele.
3. O guardião tem agora o depósito, e este servidor renova-o quando o kit muda. O **Depósito da chave** no cartão desta instância lista o **Depósito deste servidor** com cada guardião e o seu estado, e os **Depósitos que este servidor guarda** para os outros. **Criar um novo código** substitui o código. Cada guardião passa ao novo quando recebe o depósito novo.

Nesse dia precisas de três coisas: as doze palavras do teu grupo, o código de desbloqueio e acesso à interface do guardião, onde alguém tem de carregar em **Permitir**. No assistente de emergência escolhe **Através do grupo**, escolhe o depósito, pede-o e escreve o código de desbloqueio. O código é usado no servidor novo e não é enviado ao guardião. No guardião, o pedido aparece em **Depósito da chave** com quem pede, se essa instância já foi vista lá antes e quantas vezes pediu. **Permitir** entrega a cópia selada, e permitir precisa de uma palavra-passe de acesso. A instalação nova tem 30 minutos para a recolher depois do Permitir, um pedido a que ninguém responde expira ao fim de um dia, e uma instância que pede demasiadas vezes tem de esperar uma hora.

Sem o código ninguém consegue abrir a cópia, o guardião incluído. Se o código se perder, cria um novo enquanto este servidor ainda corre. Se o servidor já desapareceu, o kit de recuperação e a APP_KEY escrita à mão continuam a funcionar. O guardião pode eliminar um depósito a qualquer altura, e o dono tem então de o pedir outra vez.

## Kit de recuperação {#recovery-kit}

Esta é a peça que torna possível a recuperação de desastres mesmo quando não há nenhum BombVault a correr.

**Descarregar kit de recuperação** no cartão **Para emergências** em **Definições, Restauro** dá-te a **chave**, a **palavra-passe do restic derivada**, todos os **destinos** com o repositório de cada secção, as credenciais de que uma instalação nova precisa para os alcançar, e os **comandos exatos do restic**, para restaurares diretamente com a CLI do restic em qualquer máquina. Precisa de uma palavra-passe de acesso. O kit imprime a chave também com a encriptação desligada, porque as definições de containers e VMs são sempre seladas com ela. Depois de uma adoção, imprime a chave de backup adotada e a APP_KEY deste servidor, de que uma instalação nova só precisa para trazer as definições de volta. Um cartão **Guarda o teu kit de recuperação** na visão geral lembra-te até o teres guardado.

!!! danger "Guarda o kit de recuperação fora do servidor"
    O kit contém o segredo que desencripta os teus backups. Guarda-o num sítio seguro e separado do servidor (um gestor de palavras-passe, uma cópia impressa num cofre). Se perderes o BombVault e a `APP_KEY` sem kit de recuperação nem depósito da chave, os teus backups encriptados não podem ser recuperados.

!!! warning "O instantâneo mais recente nem sempre é o que se deve restaurar"
    Desde o restic 0.17, o `restic snapshots` mostra o tamanho de cada instantâneo. Depois de uma perda de dados, o instantâneo mais recente pode ser o esvaziado, por isso não restaures um instantâneo muito mais pequeno do que os anteriores. Depois de ransomware pode ser o encriptado, com o tamanho habitual. Se o BombVault ainda corre, vê primeiro a sua página **Anomalias**: nomeia o último bom backup, e a janela de restauração pré-seleciona-o. Uma restauração não precisa de nenhum dado de anomalias do BombVault, e a pausa da retenção só alguma vez guarda mais instantâneos.

### Selar o kit {#sealing-the-kit}

Se ligaste **Cifrar exportações com age** em **Definições, Sistema**, o kit também é selado com isso e descarrega-se como `bombvault-recovery-kit.md.age`. Está em ASCII armor em vez de binário, por isso continua a ser texto simples: colá-lo num gestor de palavras-passe ou imprimi-lo funciona exatamente como antes, o conteúdo é ilegível sem a tua chave.

!!! warning "Não guardes a chave age dentro do kit"
    Precisas da tua chave **privada** age para abrir um kit selado. Guarda-a num sítio que não dependa do próprio kit, ou ficas com duas coisas para recuperar em vez de uma. Selar vale a pena quando o kit está guardado num sítio que não controlas por inteiro (um gestor de palavras-passe partilhado, notas na nuvem, uma impressão num escritório). Um kit no teu próprio cofre já está protegido pelo cofre.

    Com a cifragem ligada e sem nenhum destinatário utilizável configurado, a transferência é recusada sem mais. O BombVault nunca recorre a entregar a chave mestra às claras.

### Se não tens o kit à mão {#if-you-do-not-have-the-kit-to-hand}

A palavra-passe não é guardada em lado nenhum, é **calculada** a partir da chave dos backups, por isso podes reproduzi-la tu mesmo apenas com a chave e uma shell:

```sh
printf 'bombvault:restic-repo'   | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r   | cut -d' ' -f1
```

Isto é HMAC-SHA256 sobre a string fixa `bombvault:restic-repo`, com a chave dos bytes brutos da `APP_KEY` em hexadecimal, impresso como 64 caracteres hexadecimais em minúsculas. O mesmo valor está no kit, listado como a palavra-passe do restic derivada. É para o dia em que o kit está onde tu não estás. Num servidor que adotou os backups de outro, usa a chave de backup adotada, que é a antiga APP_KEY desse servidor.

!!! warning "Para um repositório recebido, usa a chave da instância que ENVIA"
    Um repositório que chegou aqui por uma cópia externa foi criado pela máquina que o enviou, com a **sua** chave. Derivar da chave da máquina recetora produz uma palavra-passe que o restic rejeita, o que se lê exatamente como um repositório corrompido e não é. Este é o motivo habitual de o `restic check` num repositório recebido pedir uma palavra-passe vezes sem conta.

Como as definições de recuperação vivem **dentro** de cada repositório (`<repo>/def`, `<repo>/vm-def`), uma pasta de repositório copiada é totalmente autónoma, por isso o kit mais o repositório é tudo o que uma restauração em hardware novo precisa.

## Recuperar um despejo da base de dados {#database-dumps}

Um despejo da base de dados é um ponto de restauro próprio no repositório de containers, com a etiqueta `dbdump:<container>`, que guarda o ficheiro único `/dbdump/<container>.sql`. O BombVault lista-os, descarrega-os e importa-os no separador **Backups** do container. Abaixo estão os mesmos passos só com o restic, para o dia em que o BombVault não está lá.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

As etiquetas `dbversion:` e `dbname:` de cada despejo dizem de que versão do servidor veio e que bases de dados guarda. Um ficheiro completo termina com `-- PostgreSQL database cluster dump complete` ou `-- Dump completed`.

Importa-o para um container da mesma versão ou de uma mais recente (PostgreSQL), ou da mesma versão principal (MySQL e MariaDB), arrancado uma vez com uma pasta de dados vazia para se inicializar. O host não precisa de nenhum cliente de base de dados, o container tem um:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Para uma só base de dados de um despejo completo, o MySQL e o MariaDB aceitam `--one-database <nome>` no comando do cliente. Um despejo PostgreSQL tem uma secção por base de dados, cada uma a começar com uma linha `\connect <nome>`: copia essa secção para o seu próprio ficheiro e importa-a com `-d <nome>` depois de criares a base de dados.

!!! warning "Um despejo de root traz os utilizadores do servidor"
    Um despejo completo de MySQL ou MariaDB feito como root contém a base de dados de sistema `mysql`, por isso importá-lo substitui as contas do servidor novo, incluindo a palavra-passe do root, pelas do despejo. No PostgreSQL, `role ... already exists` para o utilizador que o container criou é esperado e inofensivo.
