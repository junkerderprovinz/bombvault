# Introdução

Esta página acompanha-o desde uma máquina Unraid acabada de instalar até ao seu primeiro backup.

## Requisitos

| Requisito | Notas |
|---|---|
| **Unraid 6.12+** | Versões anteriores não são testadas. O Unraid é o alvo principal, mas o BombVault também corre num host Docker simples e no TrueNAS Scale (consulta [Host Docker genérico](#generic-docker-host)). |
| **Localização do repo restic** | Um caminho local (recomendado: o seu array ou cache), SMB, NFS, ou qualquer backend rclone. |
| **Socket Docker** | Montado automaticamente pelo template (`/var/run/docker.sock`). |
| **Flash do Unraid** (`/boot`) | Montada por inteiro pelo template automaticamente (`/boot` para `/host/boot`). Alimenta o backup do flash e permite que um container restaurado reapareça como uma aplicação Unraid normal e editável. |
| **VMs KVM** (opcional) | O backup de VMs comunica com o libvirt por SSH, sem montagem de libvirt. Configura-o em **Definições, Integrações, SSH do anfitrião** (consulta [Configuração](configuration.md)). |
| **Conjuntos de dados ZFS** (opcional) | A mesma ligação SSH que o backup de VMs, `zfs` no host e Host Data mapeado como `/mnt` com o modo de acesso Read/Write - Slave, a predefinição do template. Consulta [Conjuntos de dados ZFS](zfs-datasets.md). |
| **Aplicação Android** (opcional) | Android 10 ou posterior, emparelhada com servidores na versão 9.7.0 ou posterior. Consulta [Aplicação Android](android.md). |

## Instalar no Unraid

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
    3. Define as variáveis obrigatórias e clique em **Apply**.

## Anfitrião Docker genérico {#generic-docker-host}

Não estás no Unraid? O BombVault também corre como contentor simples em qualquer anfitrião Docker (é também o que sustenta o suporte a contentores no TrueNAS Scale, antes de ter uma entrada própria no catálogo de aplicações).

1. Vai buscar ao repositório o ficheiro [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml), pronto a editar.
2. Define o `APP_KEY` (ver abaixo) e aponta o volume Host Data para a tua raiz de dados real. Os comentários do ficheiro explicam as duas coisas.
3. `docker compose up -d` e depois abre `https://<ip-do-anfitrião>:3443/`.

O que muda face ao Unraid:

- **Não há secção Flash.** Não existe pen de arranque para capturar ou repor, por isso a secção Flash não tem aqui nada que fazer. Em vez disso, **Adicionar** na página Backups oferece **Configuração do sistema** como sugestão de pasta: um conjunto de pastas inicial com `/etc` que revês e editas antes de guardar.
- **Não há notificações nativas do Unraid.** Os canais de notificação próprios do BombVault (webhook, alertas de falha fora do local, etc.) funcionam normalmente; só é omitido o envio específico para o sistema de notificações do Unraid, já que aqui esse sistema não existe.
- **O backup de máquinas virtuais é opcional e precisa de um anfitrião libvirtd separado, alcançável por SSH.** Vê o bloco comentado no ficheiro compose. Um anfitrião Docker genérico não traz gestor de máquinas virtuais.
- **Sem widget no painel.** O BombVault Widget é um plugin do Unraid, por isso esse passo também é omitido.
- **Encontrar os dados de um container.** Sem a convenção `appdata` do Unraid, a pasta de dados de um container é encontrada a partir dos segmentos em `DATA_ROOT_SEGMENTS`, dos volumes com nome do Docker, do diretório de trabalho de um projeto Compose e da etiqueta `bombvault.data` (consulta [Deteção das fontes de backup](configuration.md#backup-source-detection)). Os volumes com nome e a predefinição `/etc` só alcançam caminhos dentro da montagem Host Data, por isso aponta o Host Data para uma pasta ascendente comum que abranja também a raiz de dados do Docker.
- **`PLATFORM`.** Define-a como `generic` ou `truenas`. Se não for definida, o BombVault deteta o Unraid pelo seu próprio marcador na montagem do flash e trata tudo o resto como genérico, e os passos exclusivos do Unraid são omitidos em vez de serem tentados e falharem.

O **TrueNAS Scale** segue o mesmo caminho com compose; há uma entrada de catálogo preparada no repositório, mas ainda não submetida. Aí o backup de VMs precisa de `LIBVIRT_URI`, porque o libvirtd do TrueNAS escuta num socket próprio (`/run/truenas_libvirt/libvirt-sock`) que as três variáveis `LIBVIRT_*` não conseguem exprimir (consulta [Configuração](configuration.md)). Até onde isto está comprovado: o backup de zvol foi executado numa máquina TrueNAS Scale real, num zvol ligado a uma VM em execução, e `zfs snapshot`, `zfs send`, restic e `zfs receive` fizeram a ida e volta byte a byte. Um restauro completo conduzido pelo próprio BombVault ainda não foi executado em hardware TrueNAS, e esse zvol era esparso, por isso o débito com muitos gigabytes não está testado. Testa aí um restauro antes de confiares nele.

## A única definição obrigatória

A única variável que tem de definir é `APP_KEY`, um segredo hexadecimal de 32 bytes (64 caracteres hexadecimais) usado para derivar a palavra-passe do repositório restic.

Gera um em qualquer máquina:

```bash
openssl rand -hex 32
```

Cola o resultado no campo `APP_KEY` do template (Unraid), ou na variável de ambiente `APP_KEY` em `docker-compose.yml` (host Docker genérico).

!!! danger "Não perca a sua APP_KEY"
    Perder a `APP_KEY` torna os teus backups encriptados irrecuperáveis. Guarda-a num local seguro e separado do servidor. Assim que o BombVault estiver a correr, define uma palavra-passe de início de sessão e descarrega o **kit de recuperação** em **Definições, Restauro** (consulta [Externo e recuperação](offsite-recovery.md#recovery-kit)). Contém a chave, cada destino e os comandos do restic.

O template também monta o socket Docker, o flash (`/boot`) e a raiz **Host Data** (`/mnt`) por ti. As *origens* e os *destinos* de backup vivem ambos sob Host Data. Para a referência completa de variáveis e a configuração do externo, consulta [Configuração](configuration.md).

## Primeira execução

![A visão geral após o primeiro backup: o que precisa de ti, a proteção de cada secção, os destinos e o histórico.](assets/screenshots/dashboard.png)

*A visão geral após o primeiro backup: o que precisa de ti, a proteção de cada secção, os destinos e o histórico.*

1. Abre a interface web em `https://<your-unraid-ip>:3443` (certificado autoassinado logo de início). Uma instalação nova abre em **Damos-te as boas-vindas ao BombVault**, que lista o que o BombVault encontrou no servidor.
2. Escolhe **Vamos lá**. A visão geral mostra então o cartão **Primeiros passos** com três passos:
    1. **Escolhe as secções**: ativa o que o BombVault copia (Containers, VMs, Flash, Pastas, Conjuntos de dados ZFS, Auto-backup). **Escolher** abre a lista das entradas de uma secção, onde **Pausar agendamento** deixa uma de fora. As secções que ativas aqui correm todas as noites, o que alteras em **Definições, Agendamento**.
    2. **Adicionar destino**: para onde vão os backups. Uma pasta local chega para começar. O mais seguro é um segundo destino fora de casa, como uma Storage Box ou o Backblaze B2. Os repositórios locais ficam por predefinição em `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` e são criados no primeiro backup.
    3. **Primeiro backup**: depois disso corre sozinho todas as noites.
3. Abre **Backups** para ver todas as entradas numa só lista. **Fazer backup de tudo agora** inicia uma execução sobre todas as secções ativas, e **Fazer backup agora** na página de uma entrada copia apenas essa.

Já usaste o BombVault? **Restaurar a partir de um backup** na página de boas-vindas abre a página Restauração, onde o **Assistente de emergência** recupera as definições, os containers, as VMs e as pastas de um servidor perdido. Consulta [Externo e recuperação](offsite-recovery.md#emergency).

!!! tip "Opcional: escolhe uma ordem de backup"
    Se alguns containers devem ser sempre copiados antes de outros (por exemplo, uma base de dados antes da aplicação que a usa), abre o cartão **Ordem** em **Definições, Agendamento** e arrasta-os para a sequência que quiseres. As execuções agendadas e o backup total passam a segui-la; tudo o que deixares sem ordem é copiado pelo mais-em-atraso-primeiro.

!!! note "Verificação de integração com o host"
    O cartão **Integração com o host** na visão geral sonda cada montagem e CLI (socket Docker, libvirt, restic, qemu-img, rclone) e reporta quaisquer peças em falta, para que possas confirmar que o container está corretamente ligado antes de depender dele. Fica à espera em **Personalizar** até o adicionares.

## Orientar-te {#navigation}

![As definições como mosaicos, com os cartões do mosaico escolhido ao lado. Cada alteração é escrita no momento em que a fazes.](assets/screenshots/settings.png)

*As definições como mosaicos, com os cartões do mosaico escolhido ao lado. Cada alteração é escrita no momento em que a fazes.*

A barra lateral tem cinco entradas: **Visão geral**, **Anomalias** (enquanto a deteção de anomalias estiver ativa), **Backups**, **Restauração** e, por baixo de **Sair**, **Definições**. A ação principal de uma página flutua em baixo à direita, e **Mais** guarda as raras. Num telemóvel a barra tem Visão geral, Backups, Restauração e Definições, e **Mais** tem o resto.

As páginas e as definições usam toda a largura da janela. Uma janela guarda os seus botões no rodapé, com o que avança à direita. Num telemóvel, um rodapé com mais de três botões mantém a saída e a ação principal e põe os restantes em **Mais**.

- **Backups** lista cada container, VM, conjunto de pastas, a flash, cada conjunto de dados ZFS e o Auto-backup como entradas. Os mosaicos por cima da lista escolhem um tipo, e **Filtros**, **Ordenar** e **Pesquisar** restringem-na. Cada entrada tem uma página própria com três separadores: **Backups**, **Definições** e **Verificação**. Um botão de voltar à esquerda do ícone da entrada leva à lista.
- **Restauração** é uma única página para tudo: escolhe uma secção, procura uma entrada e escolhe um backup. O Assistente de emergência e as outras origens também começam aqui.
- **Definições** são catorze mosaicos: Geral, Aspeto, Agendamento, Destinos, Restauro, Notificações, Verificação, Segurança, Instâncias, Emparelhamento, Integrações, Aplicações, Sistema e Info. Prime `/` ou Ctrl+K para pesquisar todos, também pelos nomes que as páginas tinham em versões anteriores. Arrasta os mosaicos para a tua própria ordem em **Aspeto**. Onde uma coluna de mosaicos ao lado da página a apertaria, como numa janela com cerca de 1200 píxeis de largura, os mosaicos passam a uma faixa por cima da página que se desloca para os lados. Um telemóvel mostra a mesma faixa.
- **Destinos** e **Instâncias** são mosaicos de definições com páginas próprias. **Mostrar também na barra lateral** no mosaico deles fixa-os à barra lateral.

Nada tem um botão Guardar e nada se desdobra: as coisas raras têm um cartão próprio ou uma janela, e as explicações estão nos balões (i). Depois de uma atualização, as notas de versão abrem-se sozinhas, e pontos verdes marcam o que mudou. Um clique esconde um ponto. **Novidades** no cartão Versão em **Definições, Info** conta o que ainda não viste e abre as notas de versão e a lista de cada alteração com **Ir para lá**, como dois separadores de uma janela.

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

A compilação da interface também é necessária para `go run`. O repositório só regista um marcador vazio em `web/dist`, por isso sem `npm --prefix web run build` o binário não incorpora nada e responde `500 SPA index not found`, o que é normal. Docker, libvirt e Unraid não podem ser testados em CI, por isso verifica as montagens, o restic e a ligação SSH das VMs num host real com o cartão **Integração com o host** da visão geral (ou `GET /api/spike`) antes de abrires um pull request.

## Passos seguintes

- Percorra as **[Funcionalidades](features.md)** completas.
- Leve todos os servidores do seu grupo para o telemóvel com a **[Aplicação Android](android.md)**.
- Adiciona um ou mais destinos fora de casa (cada secção pode copiar para vários ao mesmo tempo) e guarda o teu kit de recuperação, como descrito em **[Externo e recuperação](offsite-recovery.md)**.
- A clonar uma configuração ou a mudar para uma máquina nova? Leve toda a sua configuração consigo com o cartão **Exportar / importar configurações**. Consulta [Configuração](configuration.md#portable-settings-export-and-import).
- Encontrou um obstáculo? Consulta **[Resolução de problemas](troubleshooting.md)**.
