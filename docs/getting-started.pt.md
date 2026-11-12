# Introdução

Esta página acompanha-te desde uma máquina Unraid acabada de instalar até ao teu primeiro backup.

## Requisitos {#requirements}

| Requisito | Notas |
|---|---|
| **Unraid 6.12+** | Versões anteriores não são testadas. O Unraid é o alvo principal, mas o BombVault também corre num host Docker simples e no TrueNAS Scale (consulta [Anfitrião Docker genérico](#generic-docker-host) e [TrueNAS SCALE](#truenas)). |
| **Localização do repo restic** | Um caminho local (recomendado: o teu array ou cache), SMB, NFS, ou qualquer backend rclone. |
| **Socket Docker** | Montado automaticamente pelo template (`/var/run/docker.sock`). |
| **Flash do Unraid** (`/boot`) | Montada por inteiro pelo template automaticamente (`/boot` para `/host/boot`). Alimenta o backup do flash e permite que um container restaurado reapareça como uma aplicação Unraid normal e editável. |
| **VMs KVM** (opcional) | O backup de VMs comunica com o libvirt por SSH, sem montagem de libvirt. Configura-o em **Definições, Integrações, SSH do anfitrião** (consulta [Configuração](configuration.md)). |
| **Conjuntos de dados ZFS** (opcional) | A mesma ligação SSH que o backup de VMs, `zfs` no host e Host Data mapeado como `/mnt` com o modo de acesso Read/Write - Slave, a predefinição do template. Consulta [Conjuntos de dados ZFS](zfs-datasets.md). |
| **Aplicação Android** (opcional) | Android 10 ou posterior, emparelhada com servidores na versão 9.7.0 ou posterior. Consulta [Aplicação Android](android.md). |

## Instalar no Unraid {#install-on-unraid}

O caminho mais fácil são as **Community Applications**.

1. Abre o separador **Apps** no Unraid.
2. Pesquisa por **BombVault**.
3. Clica em **Install**, define as variáveis obrigatórias (abaixo) e aplica.

!!! tip "Instalação manual do template"
    Se preferires adicionar o template à mão:

    1. Vai a **Docker, Add Container, Template repositories** e adiciona:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Pesquisa por **BombVault** em Templates.
    3. Define as variáveis obrigatórias e clica em **Apply**.

## Anfitrião Docker genérico {#generic-docker-host}

Não estás no Unraid? O BombVault também corre como container simples em qualquer anfitrião Docker. O TrueNAS SCALE tem os seus próprios passos em [TrueNAS SCALE](#truenas).

1. Vai buscar ao repositório o ficheiro [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml), pronto a editar.
2. Define o `APP_KEY` (vê abaixo) e aponta o volume Host Data para a tua raiz de dados real. Os comentários do ficheiro explicam as duas coisas.
3. `docker compose up -d` e depois abre `https://<ip-do-anfitrião>:3443/`.

O que muda face ao Unraid:

- **Não há secção Flash.** Não existe pen de arranque para captar ou repor, por isso a secção Flash não tem aqui nada que fazer. Em vez disso, quando o Host Data alcança o `/etc` do anfitrião, **Adicionar** na página Backups oferece **Configuração do sistema** como sugestão de pasta: um conjunto de pastas `/etc` inicial que revês e editas antes de guardar.
- **Não há notificações nativas do Unraid.** Os canais de notificação próprios do BombVault (webhook, alertas de falha das cópias externas, etc.) funcionam normalmente. Só é omitido o envio específico para o sistema de notificações do Unraid, já que aqui esse sistema não existe.
- **O backup de VMs é opcional e precisa de um anfitrião libvirtd separado, alcançável por SSH.** Vê o bloco comentado no ficheiro compose. Um anfitrião Docker genérico não traz gestor de máquinas virtuais.
- **Sem mosaico no painel.** O TollRoll é um plugin do Unraid, por isso esse passo também é omitido.
- **Encontrar os dados de um container.** Sem a convenção `appdata` do Unraid, a pasta de dados de um container é encontrada a partir dos segmentos em `DATA_ROOT_SEGMENTS`, dos volumes com nome do Docker, do diretório de trabalho de um projeto Compose e da etiqueta `bombvault.data` (consulta [Deteção das origens de backup](configuration.md#backup-source-detection)). Os volumes com nome e a predefinição `/etc` só alcançam caminhos dentro da montagem Host Data, por isso aponta o Host Data para uma pasta ascendente comum que abranja também a raiz de dados do Docker.
- **`PLATFORM`.** Define-a como `generic` ou `truenas`. Se não for definida, o BombVault deteta o Unraid pelo seu próprio marcador na montagem do flash e trata tudo o resto como genérico, e os passos exclusivos do Unraid são omitidos em vez de serem tentados e falharem.

### TrueNAS SCALE {#truenas}

O BombVault ainda não está no catálogo de aplicações do TrueNAS, por isso instala-o como aplicação personalizada: **Apps, Discover Apps** e depois **Install via YAML** no menu ao lado de **Custom App**. O YAML abaixo define o mesmo que a entrada de catálogo na pasta `truenas-apps` do repositório. Substitui `tank` pelo teu pool e preenche o `APP_KEY` como explica a secção seguinte.

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    hostname: bombvault
    restart: unless-stopped
    cap_drop: [ALL]
    cap_add: [CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETPCAP, SETUID, SYS_CHROOT]
    environment:
      APP_KEY: ""
      PLATFORM: truenas
      HOST_SOURCE_ROOT: /mnt/tank
      HOST_MOUNT_ROOT: /mnt/tank
      # For VM backups:
      # LIBVIRT_URI: qemu+ssh://root@<truenas-ip>/system?socket=/run/truenas_libvirt/libvirt-sock
    volumes:
      - /mnt/tank/apps/bombvault:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - type: bind
        source: /mnt/tank
        target: /mnt/tank
        bind:
          propagation: rslave
    ports:
      - "30528:3443"
```

Depois abre `https://<truenas-ip>:30528/`. O que está em [O que muda face ao Unraid](#generic-docker-host) vale também aqui, e no TrueNAS:

- **Capabilities.** O BombVault precisa das sete acima. Sem `SETPCAP`, os backups de VMs falham, e sem `SYS_CHROOT` cada container é parado durante todo o seu backup em vez de ser lido de um instantâneo.
- **Host Data** é um caminho do anfitrião num conjunto de dados, aqui `/mnt/tank`, montado no mesmo caminho dentro da aplicação. É por isso que `HOST_SOURCE_ROOT` e `HOST_MOUNT_ROOT` o indicam ambas. O `rslave` passa as montagens que o anfitrião faz mais tarde, de que os backups de conjuntos de dados ZFS precisam.
- **Os dados num ixVolume não entram no backup.** Uma aplicação cujo armazenamento é um ixVolume guarda os seus dados em `/mnt/.ix-apps`, fora do Host Data. O BombVault guarda só a definição da aplicação, e a execução indica as pastas que deixou de fora. Para cada aplicação cujos dados queiras no backup, escolhe armazenamento **Host Path** num conjunto de dados abaixo do Host Data.
- **Os backups de VMs e de conjuntos de dados ZFS** chegam ao TrueNAS por SSH como root. Ativa o serviço SSH em **System, Services** e adiciona a chave pública de **Definições, Integrações, SSH do anfitrião** ao utilizador root em **Credentials, Users**. Os backups de VMs precisam também de `LIBVIRT_URI` como acima, porque o libvirtd do TrueNAS escuta num socket próprio. Sem `LIBVIRT_URI` nem `LIBVIRT_HOST` definidos, o BombVault usa como anfitrião o gateway predefinido da aplicação, já que `host.docker.internal` não resolve numa aplicação do TrueNAS. O [guia de backup de VMs](vm-backup-ssh-setup.md#truenas-scale) cobre os pormenores, incluindo VMs em zvols.
- **Uma réplica ZFS usa outra chave.** Uma réplica num segundo servidor ZFS entra com a chave que a janela da réplica mostra, e não com a chave SSH do anfitrião. Adiciona essa no servidor que recebe.
- **Uma aplicação apagada volta em dois passos.** Primeiro reinstala-a no TrueNAS com o mesmo nome, o que volta a criar a sua rede, e depois restaura os seus dados no BombVault. Até lá, a verificação antes do restauro indica a rede em falta, e o restauro recusa-se antes de alterar o que quer que seja.
- **Ao lado do original** cria um container Docker simples. Funciona, mas o ecrã Apps do TrueNAS não o mostra.
- **Os despejos de bases de dados** funcionam em montagens de conjuntos de dados. Uma importação para uma pasta de dados que é um conjunto de dados próprio move o conteúdo antigo para uma pasta ao lado, por isso o conjunto de dados acima precisa de espaço para ele.

### ZimaOS, CasaOS e outros anfitriões {#other-hosts}

- **ZimaOS e CasaOS:** adiciona a [loja da comunidade](https://github.com/junkerderprovinz/zimaos-apps#install) e instala o BombVault a partir dela. Corre aí como `PLATFORM=generic` com o Host Data em `/DATA`. O ZimaOS guarda os dados das aplicações em `/DATA/AppData/<app>`, que o BombVault encontra sozinho, já que `DATA_ROOT_SEGMENTS` corresponde a `appdata` em qualquer capitalização.
- **MOS:** instala o BombVault a partir do [MOS Hub](https://mos-official.net/apps.html?search=BombVault). Corre aí como `PLATFORM=generic` com o Host Data em `/mnt/cache`.
- **Synology:** no Container Manager, cria um projeto a partir do ficheiro compose (DSM 7.2 ou mais recente).
- **Proxmox:** corre-o numa VM ou num container LXC que tenha Docker.

Sem comentários, o ficheiro compose genérico é curto. Substitui `./data` pela pasta do anfitrião que guarda os dados dos teus containers, define `HOST_SOURCE_ROOT` com esse mesmo caminho e preenche o `APP_KEY`:

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    restart: unless-stopped
    hostname: bombvault
    container_name: bombvault
    volumes:
      - ./config:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/host/user
    environment:
      APP_KEY: ""
      PLATFORM: generic
      HOST_SOURCE_ROOT: /host/user
    ports:
      - "3443:3443"
```

Para copiar também os volumes com nome do Docker, monta uma pasta que guarde tanto os teus dados como a raiz de dados do Docker (`docker info -f '{{.DockerRootDir}}'`, normalmente `/var/lib/docker`), tal como o template do Unraid monta todo o `/mnt`.

## A única definição obrigatória {#the-one-required-setting}

A única variável que tens de definir é o `APP_KEY`, um segredo hexadecimal de 32 bytes (64 caracteres hexadecimais) usado para derivar a palavra-passe do repositório restic.

Gera um em qualquer máquina:

```bash
openssl rand -hex 32
```

Cola o resultado no campo `APP_KEY` do template (Unraid), ou na variável de ambiente `APP_KEY` em `docker-compose.yml` (anfitrião Docker genérico).

!!! danger "Não percas a tua APP_KEY"
    Perder a `APP_KEY` torna os teus backups encriptados irrecuperáveis. Guarda-a num local seguro e separado do servidor. Assim que o BombVault estiver a correr, define uma palavra-passe de acesso e descarrega o **kit de recuperação** em **Definições, Restauro** (consulta [Externo e recuperação](offsite-recovery.md#recovery-kit)). Contém a chave, cada destino e os comandos do restic.

O template também monta por ti o socket Docker, o flash (`/boot`) e a raiz **Host Data** (`/mnt`). As *origens* e os *destinos* de backup vivem ambos sob o Host Data. Para a referência completa de variáveis e a configuração das cópias externas, consulta [Configuração](configuration.md).

## Primeira execução {#first-run}

![A página de boas-vindas de uma instalação nova, com o que o BombVault encontrou no servidor.](assets/screenshots/getting-started/welcome-dark.png#only-dark)
![A página de boas-vindas de uma instalação nova, com o que o BombVault encontrou no servidor.](assets/screenshots/getting-started/welcome-light.png#only-light)

1. Abre a interface web em `https://<ip-do-teu-unraid>:3443` (certificado autoassinado logo de início). Uma instalação nova abre em **Damos-te as boas-vindas ao BombVault**, que lista o que o BombVault encontrou no servidor.
2. Escolhe **Vamos lá**. A visão geral mostra então o cartão **Primeiros passos** com três passos:
    1. **Escolhe as secções**: liga o que o BombVault copia (Containers, VMs, Flash, Pastas, Conjuntos de dados ZFS, Auto-backup). **Escolher** abre a lista das entradas de uma secção, onde **Pausar agendamento** deixa uma de fora. As secções que ligas aqui correm todas as noites, o que alteras em **Definições, Agendamento**.
    2. **Adicionar destino**: um segundo lugar para os backups. Cada secção já tem uma pasta neste servidor, `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}`, criada no primeiro backup. O mais seguro é um segundo destino fora de casa, como uma Storage Box ou o Backblaze B2.
    3. **Primeiro backup**: depois disso corre sozinho todas as noites. O cartão fica até o primeiro backup terminar.
3. Abre **Backups** para ver todas as entradas numa só lista. **Fazer backup de tudo agora** inicia uma execução sobre todas as secções ligadas, e **Fazer backup agora** na página de uma entrada copia apenas essa.

![O cartão Primeiros passos com os seus três passos, o segundo aberto.](assets/screenshots/getting-started/first-steps-dark.png#only-dark)
![O cartão Primeiros passos com os seus três passos, o segundo aberto.](assets/screenshots/getting-started/first-steps-light.png#only-light)

Já usaste o BombVault? **Restaurar a partir de um backup** na página de boas-vindas abre a página Restauração, onde o **Assistente de emergência** recupera as definições, os containers, as VMs e as pastas de um servidor perdido. Vê [Externo e recuperação](offsite-recovery.md#emergency).

!!! tip "Opcional: escolhe uma ordem de backup"
    Se alguns containers devem ser sempre copiados antes de outros (por exemplo, uma base de dados antes da aplicação que a usa), abre o cartão **Ordem** em **Definições, Agendamento** e arrasta-os para a sequência que quiseres. As execuções agendadas e o backup total passam a segui-la. Tudo o que deixares sem ordem é copiado pelo mais atrasado primeiro.

## Exemplo: o teu primeiro backup de containers, de ponta a ponta {#example-first-backup}

Isto percorre uma máquina Unraid nova com alguns containers, entre eles o `plex`, até o primeiro backup estar feito e saberes onde o encontrar. Demora cerca de dez minutos mais o tempo do primeiro backup.

1. **Instalar.** No separador **Apps** do Unraid, pesquisa por **BombVault** e carrega em **Install**.
2. **Definir a chave.** Num computador, corre `openssl rand -hex 32` e cola os 64 caracteres no campo `APP_KEY` do template. Guarda já uma cópia no teu gestor de palavras-passe, antes de mais nada. Carrega em **Apply**.
3. **Abri-lo.** Vai a `https://<ip-do-teu-unraid>:3443` e aceita o certificado autoassinado. **Damos-te as boas-vindas ao BombVault** lista o que encontrou, por exemplo 12 containers, 2 VMs e a unidade flash.
4. **Vamos lá.** A visão geral abre com o cartão **Primeiros passos**.
5. **Escolhe as secções.** Os **Containers** vêm ligados desde o início. Liga o que mais quiseres, por exemplo o **Flash**. **Escolher** abre a lista de containers que encontrou, onde **Pausar agendamento** deixa um de fora, por exemplo um container de teste que não te importa. As secções ligadas correm todas as noites no backup total.
6. **Adicionar destino.** Cada secção já tem a sua própria pasta neste servidor, por baixo de `/mnt/user/bombvault`, por isso o primeiro backup tem para onde ir. Este passo acrescenta um segundo lugar, e o mais seguro é fora de casa. Podes fazê-lo agora com o [exemplo S3](offsite-recovery.md#example-s3), ou voltar a ele quando o primeiro backup estiver feito. O cartão fica até lá.
7. **Primeiro backup.** Abre **Backups** e carrega em **Fazer backup de tudo agora**, ou espera pela noite. O anel no cabeçalho de estado passa a um anel de progresso e conta as entradas, por exemplo **A fazer backup agora: 3 de 12**. Cada container é parado enquanto os seus dados são lidos e volta a arrancar logo a seguir. Um container cujos dados estão em ZFS fica em baixo só os poucos segundos de um instantâneo. O primeiro backup lê tudo, por isso demora mais. Os seguintes guardam só o que mudou.
8. **Verificar.** Quando o anel diz **Tudo copiado**, abre o **Histórico** na visão geral: cada container tem uma linha verde em **Execuções**. Abre **Backups**, clica em `plex` e olha para o seu separador **Backups**: o primeiro instantâneo está lá, com **Local** como o lugar que o guarda. Uns minutos depois, o seu separador **Verificação** mostra a verificação de restauro, que restaurou uma amostra do backup e a leu de volta.
9. **Proteger a chave.** Abre **Definições, Segurança** e define uma palavra-passe de acesso. Depois abre **Definições, Restauro** e carrega em **Descarregar kit de recuperação** em **Para emergências**. Guarda o ficheiro fora do servidor, junto à cópia do `APP_KEY` do passo 2. O lembrete **Guarda o teu kit de recuperação** na visão geral desaparece quando carregares em **Já o guardei em segurança**.

A partir de agora os containers são copiados todas as noites. Bons passos seguintes, por esta ordem: um [destino externo](offsite-recovery.md#example-s3), para que um servidor morto não leve os backups consigo, depois as [notificações](notifications.md), para que uma falha te chegue, e depois uma vista de olhos aos [Agendamentos](schedules.md) se as 03:00 não te servirem.

![O cabeçalho de estado enquanto o primeiro backup corre, com o anel de progresso e as suas estações.](assets/screenshots/getting-started/status-ring-dark.png#only-dark)
![O cabeçalho de estado enquanto o primeiro backup corre, com o anel de progresso e as suas estações.](assets/screenshots/getting-started/status-ring-light.png#only-light)

!!! note "Verificação de integração com o host"
    O cartão **Integração com o host** na visão geral sonda cada montagem e CLI (socket Docker, libvirt, restic, qemu-img, rclone) e comunica quaisquer peças em falta, para que possas confirmar que o container está corretamente ligado antes de depender dele. Fica à espera em **Personalizar** até o adicionares.

## Orientar-te {#navigation}

![As definições como mosaicos, com os cartões do mosaico escolhido ao lado. Cada alteração é escrita no momento em que a fazes.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![As definições como mosaicos, com os cartões do mosaico escolhido ao lado. Cada alteração é escrita no momento em que a fazes.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

A barra lateral tem cinco entradas: **Visão geral**, **Anomalias** (enquanto a deteção de anomalias estiver ativa), **Backups**, **Restauração** e, por baixo de **Sair**, **Definições**. A ação principal de uma página flutua em baixo à direita, e **Mais** guarda as raras. Num telemóvel a barra tem Visão geral, Backups, Restauração e Definições, e **Mais** tem o resto.

As páginas e as definições usam toda a largura da janela. Uma janela guarda os seus botões no rodapé, com o que avança à direita. Num telemóvel, um rodapé com mais de três botões mantém a saída e a ação principal e põe os restantes em **Mais**.

![Num telemóvel, a barra em baixo com Mais aberto.](assets/screenshots/getting-started/phone-more-dark.png#only-dark)
![Num telemóvel, a barra em baixo com Mais aberto.](assets/screenshots/getting-started/phone-more-light.png#only-light)

- A **Visão geral** mostra se está tudo com backup e o que precisa de ti. Vê [Visão geral](overview.md).
- **Backups** lista cada container, VM, conjunto de pastas, a flash, cada conjunto de dados ZFS e o auto-backup como entradas. Os mosaicos por cima da lista escolhem um tipo, e **Filtros**, **Ordenar** e **Pesquisar** restringem-na. Cada entrada tem uma página própria com três separadores: **Backups**, **Definições** e **Verificação**. Um botão de voltar à esquerda do ícone da entrada leva à lista. Vê [Backups e entradas](backups.md).
- **Restauração** é uma única página para tudo: escolhe uma secção, procura uma entrada e escolhe um backup. O assistente de emergência e as outras origens também começam aqui. Vê [Restauração](restore.md).
- As **Definições** são catorze mosaicos: Geral, Aspeto, Agendamento, Destinos, Restauro, Notificações, Verificação, Segurança, Instâncias, Emparelhamento, Integrações, Aplicações, Sistema e Info. Vê [Definições](settings.md). Carrega em `/` ou Ctrl+K para pesquisar em todos, também pelos nomes que as páginas tinham em versões anteriores. Arrasta os mosaicos para a tua própria ordem em **Aspeto**. Onde uma coluna de mosaicos ao lado da página a apertaria, como numa janela com cerca de 1200 píxeis de largura, os mosaicos passam a uma faixa por cima da página que desliza para os lados. Um telemóvel mostra a mesma faixa.
- **Destinos** e **Instâncias** são mosaicos de definições com páginas próprias. **Mostrar também na barra lateral** no mosaico deles fixa-os à barra lateral.

Nada tem um botão Guardar e nada se desdobra: as coisas raras têm um cartão próprio ou uma janela, e as explicações estão nos balões (i). Depois de uma atualização abre-se sozinha uma janela com um cartão para cada versão desde a tua última, e pontos verdes marcam o que mudou. Um clique esconde um ponto. **Novo** no cartão Versão em **Definições, Info** conta o que ainda não viste e volta a abrir essa janela, com as notas de cada versão e as suas alterações com **Ir para lá**.

## Compilar a partir do código-fonte {#build-from-source}

O BombVault é um único binário Go estático que serve uma API JSON e uma interface React incorporada. Compila primeiro a interface e depois corre o binário:

```bash
npm --prefix web ci
npm --prefix web run build     # writes web/dist, which the binary embeds
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # unit and integration tests, with a real restic round trip
golangci-lint run ./...
go run ./cmd/bombvault         # serves https://localhost:3443 with a self-signed certificate
```

A compilação da interface também é necessária para o `go run`. O repositório só regista um marcador vazio em `web/dist`, por isso sem `npm --prefix web run build` o binário não incorpora nada e responde `500 SPA index not found`, o que é normal. O Docker, o libvirt e o Unraid não podem ser testados em CI, por isso verifica as montagens, o restic e a ligação SSH das VMs num host real com o cartão **Integração com o host** da visão geral (ou `GET /api/spike`) antes de abrires um pull request.

## Passos seguintes {#next-steps}

- Junta um ou mais destinos fora de casa (cada secção pode copiar para vários ao mesmo tempo) e guarda o teu kit de recuperação, como descrito em **[Externo e recuperação](offsite-recovery.md)**. O [exemplo S3](offsite-recovery.md#example-s3) percorre um.
- Garante que uma falha te chega com as **[Notificações](notifications.md)**.
- Vê quando as coisas correm em **[Agendamentos](schedules.md)**.
- Leva todos os servidores do teu grupo no telemóvel com a **[Aplicação Android](android.md)**.
- Os **[exemplos práticos](guides.md)** percorrem as tarefas mais comuns, e as **[Funcionalidades](features.md)** listam tudo o que o BombVault faz.
- A clonar uma configuração ou a mudar para uma máquina nova? Leva toda a tua configuração contigo com o cartão **Exportar / importar configurações**. Consulta [Configuração](configuration.md#portable-settings-export-and-import).
- Encontraste um obstáculo? Consulta a **[Resolução de problemas](troubleshooting.md)**.
