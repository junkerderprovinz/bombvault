# Introdução

Esta página acompanha-o desde uma máquina Unraid acabada de instalar até ao seu primeiro backup.

## Requisitos

| Requisito | Notas |
|---|---|
| **Unraid 6.12+** | Versões anteriores não são testadas. O Unraid é o alvo principal, mas o BombVault também corre num host Docker simples e no TrueNAS Scale (consulte [Host Docker genérico](#generic-docker-host)). |
| **Localização do repo restic** | Um caminho local (recomendado: o seu array ou cache), SMB, NFS, ou qualquer backend rclone. |
| **Socket Docker** | Montado automaticamente pelo template (`/var/run/docker.sock`). |
| **Flash do Unraid** (`/boot`) | Montada por inteiro pelo template automaticamente (`/boot` para `/host/boot`). Alimenta o backup do flash e permite que um container restaurado reapareça como uma aplicação Unraid normal e editável. |
| **VMs KVM** (opcional) | O backup de VMs comunica com o libvirt por SSH, sem montagem de libvirt. Configure-o em Definições (consulte [Configuração](configuration.md)). |
| **Conjuntos de dados ZFS** (opcional) | A mesma ligação SSH que o backup de VMs, `zfs` no host e Host Data mapeado como `/mnt` com o modo de acesso Read/Write - Slave, a predefinição do template. Consulte [Conjuntos de dados ZFS](zfs-datasets.md). |
| **Aplicação Android** (opcional) | Android 10 ou posterior, emparelhada com servidores na versão 9.7.0 ou posterior. Consulte [Aplicação Android](android.md). |

## Instalar no Unraid

O caminho mais fácil são as **Community Applications**.

1. Abra o separador **Apps** no Unraid.
2. Pesquise por **BombVault**.
3. Clique em **Install**, defina as variáveis obrigatórias (abaixo) e aplique.

!!! tip "Instalação manual do template"
    Se preferir adicionar o template à mão:

    1. Vá a **Docker, Add Container, Template repositories** e adicione:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Pesquise por **BombVault** em Templates.
    3. Defina as variáveis obrigatórias e clique em **Apply**.

## Anfitrião Docker genérico {#generic-docker-host}

Não estás no Unraid? O BombVault também corre como contentor simples em qualquer anfitrião Docker (é também o que sustenta o suporte a contentores no TrueNAS Scale, antes de ter uma entrada própria no catálogo de aplicações).

1. Vai buscar ao repositório o ficheiro [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml), pronto a editar.
2. Define o `APP_KEY` (ver abaixo) e aponta o volume Host Data para a tua raiz de dados real: os comentários do ficheiro explicam as duas coisas.
3. `docker compose up -d` e depois abre `https://<ip-do-anfitrião>:3443/`.

O que muda face ao Unraid:

- **Não há domínio flash/USB.** Não existe pen de arranque para capturar ou repor, por isso o domínio Flash nas definições não tem aqui nada que fazer. Em vez disso, o domínio Pastas oferece a sugestão de um clique **Adicionar predefinição: configuração do sistema anfitrião** (um conjunto inicial de ficheiros `/etc` que revês e editas antes de guardar), como equivalente genérico prático.
- **Não há notificações nativas do Unraid.** Os canais de notificação próprios do BombVault (webhook, alertas de falha fora do local, etc.) funcionam normalmente; só é omitido o envio específico para o sistema de notificações do Unraid, já que aqui esse sistema não existe.
- **A cópia de máquinas virtuais é opcional e precisa de um anfitrião libvirtd separado, alcançável por SSH.** Vê o bloco comentado no ficheiro compose. Um anfitrião Docker genérico não traz gestor de máquinas virtuais.
- **Sem widget no painel.** O BombVault Widget é um plugin do Unraid, por isso esse passo também é omitido.
- **Encontrar os dados de um container.** Sem a convenção `appdata` do Unraid, a pasta de dados de um container é encontrada a partir dos segmentos em `DATA_ROOT_SEGMENTS`, dos volumes com nome do Docker, do diretório de trabalho de um projeto Compose e da etiqueta `bombvault.data` (consulte [Deteção das fontes de cópia](configuration.md#backup-source-detection)). Os volumes com nome e a predefinição `/etc` só alcançam caminhos dentro da montagem Host Data, por isso aponte o Host Data para uma pasta ascendente comum que abranja também a raiz de dados do Docker.
- **`PLATFORM`.** Defina-a como `generic` ou `truenas`. Se não for definida, o BombVault deteta o Unraid pelo seu próprio marcador na montagem do flash e trata tudo o resto como genérico, e os passos exclusivos do Unraid são omitidos em vez de serem tentados e falharem.

O **TrueNAS Scale** segue o mesmo caminho com compose; há uma entrada de catálogo preparada no repositório, mas ainda não submetida. Aí o backup de VMs precisa de `LIBVIRT_URI`, porque o libvirtd do TrueNAS escuta num socket próprio (`/run/truenas_libvirt/libvirt-sock`) que as três variáveis `LIBVIRT_*` não conseguem exprimir (consulte [Configuração](configuration.md)). Até onde isto está comprovado: o backup de zvol foi executado numa máquina TrueNAS Scale real, num zvol ligado a uma VM em execução, e `zfs snapshot`, `zfs send`, restic e `zfs receive` fizeram a ida e volta byte a byte. Um restauro completo conduzido pelo próprio BombVault ainda não foi executado em hardware TrueNAS, e esse zvol era esparso, por isso o débito com muitos gigabytes não está testado. Teste aí um restauro antes de confiar nele.

## A única definição obrigatória

A única variável que tem de definir é `APP_KEY`, um segredo hexadecimal de 32 bytes (64 caracteres hexadecimais) usado para derivar a palavra-passe do repositório restic.

Gere um em qualquer máquina:

```bash
openssl rand -hex 32
```

Cole o resultado no campo `APP_KEY` do template (Unraid), ou na variável de ambiente `APP_KEY` em `docker-compose.yml` (host Docker genérico).

!!! danger "Não perca a sua APP_KEY"
    Perder a `APP_KEY` torna os seus backups encriptados irrecuperáveis. Guarde-a num local seguro e separado do servidor. Assim que o BombVault estiver a correr, use o seu **kit de recuperação da chave de encriptação** com um clique (consulte [Externo e recuperação](offsite-recovery.md)) para guardar o pacote de recuperação completo.

O template também monta o socket Docker, o flash (`/boot`) e a raiz **Host Data** (`/mnt`) por si. As *origens* e os *destinos* de backup vivem ambos sob Host Data. Para a referência completa de variáveis e a configuração do externo, consulte [Configuração](configuration.md).

## Primeira execução

![O painel após a primeira cópia: o que está protegido, o que corre a seguir e um registo ao vivo.](assets/screenshots/dashboard.png)

*O painel após a primeira cópia: o que está protegido, o que corre a seguir e um registo ao vivo.*

1. Abra a interface web em `https://<your-unraid-ip>:3443` (certificado autoassinado logo de início).
2. Em **Definições**, ative os domínios de backup que pretende (Containers, VMs, Flash, Auto-backup, Pastas, Conjuntos de dados ZFS) e escolha uma cor de destaque.
3. No separador **Containers**, escolha um container e clique em **Fazer backup agora** para criar o seu primeiro ponto de restauro. Os caminhos do repositório assumem por predefinição `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` e são criados no primeiro backup.
4. Configure o agendamento em **Definições, Agendamentos**. Existe um *Incluir tudo no agendamento* com um clique para containers e VMs.

!!! tip "Opcional: escolha uma ordem de backup"
    Se alguns containers devem ser sempre copiados antes de outros (por exemplo, uma base de dados antes da aplicação que a usa), abra o painel **Ordem dos backups** na página Containers e arraste-os para a sequência que quiser. As execuções agendadas e de seleção múltipla passam a segui-la; tudo o que deixar sem ordem é copiado pelo mais-em-atraso-primeiro, como antes.

!!! note "Verificação de integração com o host"
    Abra `/spike` na interface web depois de o container arrancar. Sonda cada montagem e CLI (socket Docker, libvirt, restic, qemu-img, rclone) e reporta quaisquer peças em falta, para que possa confirmar que o container está corretamente ligado antes de depender dele.

## Simples vs Avançado

![As definições não têm botão Guardar: cada alteração é escrita no momento.](assets/screenshots/settings.png)

*As definições não têm botão Guardar: cada alteração é escrita no momento.*

Por predefinição, a interface mostra apenas o essencial (fazer backup, restaurar, agendar). Use o interruptor **Vista simples / Vista avançada** na barra lateral para revelar os controlos de especialista: retenção, cópia externa, hooks pré/pós, restauro ao nível do ficheiro, notificações, métricas Prometheus e as ferramentas de integridade/manutenção. É uma preferência por navegador e está desligada por predefinição, para que os recém-chegados tenham uma interface limpa e os utilizadores avançados tenham tudo.

## Compilar a partir do código-fonte {#build-from-source}

O BombVault é um único binário Go estático que serve uma API JSON e uma interface React incorporada. Compile primeiro a interface e depois corra o binário:

```bash
npm --prefix web ci
npm --prefix web run build     # writes web/dist, which the binary embeds
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # unit and integration tests, with a real restic round trip
golangci-lint run ./...
go run ./cmd/bombvault         # serves https://localhost:3443 with a self-signed certificate
```

A compilação da interface também é necessária para `go run`. O repositório só regista um marcador vazio em `web/dist`, por isso sem `npm --prefix web run build` o binário não incorpora nada e responde `500 SPA index not found`, o que é normal. Docker, libvirt e Unraid não podem ser testados em CI, por isso verifique as montagens, o restic e a ligação SSH das VMs num host real com a verificação de integração com o host (`/spike`) antes de abrir um pull request.

## Passos seguintes

- Percorra as **[Funcionalidades](features.md)** completas.
- Leve todos os servidores do seu grupo para o telemóvel com a **[Aplicação Android](android.md)**.
- Adicione uma ou mais réplicas **[Externo e recuperação](offsite-recovery.md)** (cada domínio pode enviar para vários destinos de uma só vez) e guarde o seu kit de recuperação.
- A clonar uma configuração ou a mudar para uma máquina nova? Leve toda a sua configuração consigo com o cartão **Exportar / importar configurações**. Consulte [Configuração](configuration.md#portable-settings-export-and-import).
- Encontrou um obstáculo? Consulte **[Resolução de problemas](troubleshooting.md)**.
