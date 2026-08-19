# BombVault

**Os seus dados do Unraid, selados num cofre. Largue um backup. Detone um restauro.**

O BombVault é uma aplicação web self-hosted e nativa do Unraid para **backup e recuperação total de desastres** dos seus containers Docker e VMs KVM/libvirt. Corre como um único container Docker multi-arch, oferece-lhe uma interface web moderna que segue a preferência de tema claro/escuro do seu sistema e trata de todo o ciclo de vida: fazer backup, agendar, verificar e restaurar.

Os restauros são automáticos. Os containers reaparecem no separador Docker do Unraid exatamente como antes, e as VMs são redefinidas no VM Manager com os seus discos e a NVRAM UEFI reanexados. Sem reinstalações manuais, sem reconfiguração, sem dramas.

Assente em [restic](https://restic.net), por isso cada backup é deduplicado, incremental e sempre encriptado.

!!! note "Guarde a sua APP_KEY em segurança"
    O BombVault deriva a palavra-passe do repositório restic a partir de um segredo de 32 bytes chamado `APP_KEY`. Perdê-lo torna os backups encriptados irrecuperáveis. Gere um com `openssl rand -hex 32` e guarde-o num local seguro. Consulte [Configuração](configuration.md).

## O que o BombVault protege

| Domínio | O que é guardado |
|---|---|
| **Containers Docker** | Diretório appdata mais a definição do container (imagem, variáveis de ambiente, portas, etiquetas, volumes). |
| **VMs KVM / libvirt** | Imagem(ns) de disco da VM, a definição XML e a NVRAM UEFI, com backup por SSH (sem montagem de libvirt). |
| **Flash do Unraid** | Toda a pen USB flash (`/boot`): SO, licença, configuração do array, partilhas, rede e configuração de plugins. |
| **Configuração da aplicação** | O próprio `/config` do BombVault: a sua base de dados de definições, as credenciais externas e o par de chaves SSH do libvirt. |
| **Ficheiros e pastas** | **Conjuntos de ficheiros** nomeados, qualquer pasta no servidor, cada um com padrões de exclusão opcionais por conjunto. |

## O restauro é a estrela

Depois de copiar os dados de volta a partir do instantâneo restic, o BombVault reproduz a definição de container guardada contra a Docker API, para que o container reapareça no separador Docker do Unraid como se sempre lá tivesse estado (mesma imagem, mesmas definições, mesmos mapeamentos de portas). As VMs têm o seu XML redefinido por SSH e os seus discos e a NVRAM UEFI reanexados, mesmo depois de a VM ter sido eliminada.

Quando um backup para containers dependentes, estes voltam na ordem correta: o BombVault reinicia-os pela ordem `depends_on` do Compose e espera que cada um reporte saudável antes de iniciar os que dependem dele, para que nada avance à frente de uma base de dados ou de um gateway que ainda não esteja disponível. Consulte [Funcionalidades](features.md).

## Como funciona

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-domain scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

O BombVault é a camada de orquestração e de interface, não o motor de armazenamento. Todo o movimento de dados real passa pelo restic.

## Início rápido

Novo por aqui? Vá a **[Introdução](getting-started.md)** para instalar o BombVault no Unraid através das Community Applications e correr o seu primeiro backup. Depois explore as **[Funcionalidades](features.md)** completas, afine a sua **[Configuração](configuration.md)** e configure o **[Externo e recuperação](offsite-recovery.md)**.

O externo pode espalhar-se por vários destinos por domínio de uma só vez, um **painel recetor** só de leitura monitoriza essas cópias na máquina que as recebe, e pode levar toda a sua configuração para uma máquina nova com o cartão **Exportar e importar definições**. Consulte [Externo e recuperação](offsite-recovery.md) e [Configuração](configuration.md#portable-settings-export-and-import).

## Ligações

- **Código-fonte:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Thread de suporte do Unraid:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Issues:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Controlo do host equivalente a root"
    Através do socket Docker, o BombVault pode parar, remover e recriar containers e ler/escrever appdata, e para o backup de VMs inicia sessão no host por SSH para correr `virsh`. Qualquer pessoa que consiga alcançar a sua interface web tem, efetivamente, root no host. Corra o BombVault apenas numa rede de confiança e não exposta, e ative o controlo por palavra-passe opcional (Definições, Segurança) assim que estiver a usar backups externos ou imutáveis. Consulte [Configuração](configuration.md) para o modelo de segurança completo.
