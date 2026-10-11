# BombVault

**Os seus dados do Unraid, selados num cofre. Largue um backup. Detone um restauro.**

O BombVault é uma aplicação web self-hosted e nativa do Unraid para **backup e recuperação total de desastres** dos seus containers Docker e VMs KVM/libvirt. Corre como um único container Docker multi-arch, dá-te uma interface web num tema escuro, claro ou do sistema e trata de todo o ciclo de vida: fazer backup, agendar, verificar e restaurar.

Os restauros são automáticos. Os containers reaparecem no separador Docker do Unraid exatamente como antes, e as VMs são redefinidas no VM Manager com os seus discos e a NVRAM UEFI reanexados. Sem reinstalações manuais, sem reconfiguração, sem dramas.

Assente em [restic](https://restic.net), por isso cada backup é deduplicado, incremental e sempre encriptado.

!!! note "Guarda a tua APP_KEY em segurança"
    O BombVault deriva a palavra-passe do repositório restic a partir de um segredo de 32 bytes chamado `APP_KEY`. Perdê-lo, juntamente com o kit de recuperação, torna os backups encriptados irrecuperáveis. Gera um com `openssl rand -hex 32` e guarda-o num local seguro. Consulta [Configuração](configuration.md).

## O que o BombVault protege

Cada container, VM, conjunto de pastas, a flash, cada conjunto de dados ZFS e as definições do próprio BombVault são uma **entrada** com uma página própria. As entradas do mesmo tipo formam uma **secção**, que ativas em **Definições, Geral**.

| Secção | O que é guardado |
|---|---|
| **Containers** | Diretório appdata mais a definição do container (imagem, variáveis de ambiente, portas, etiquetas, volumes). |
| **VMs** | Imagem(ns) de disco da VM, a definição XML e a NVRAM UEFI, com backup por SSH (sem montagem de libvirt). |
| **Flash** | Toda a pen USB flash (`/boot`): SO, licença, configuração do array, partilhas, rede e configuração de plugins. |
| **Auto-backup** | O próprio `/config` do BombVault: a base de dados de definições, os inícios de sessão dos destinos e o par de chaves SSH do libvirt. |
| **Pastas** | Qualquer pasta no servidor como conjunto de pastas, cada um com padrões de exclusão opcionais. |
| **Conjuntos de dados ZFS** | Um conjunto de dados com todos os que estão abaixo dele, lido de um instantâneo ZFS e guardado como uma pasta. Vê [Conjuntos de dados ZFS](zfs-datasets.md). |

## O restauro é a estrela

Depois de copiar os dados de volta a partir do instantâneo restic, o BombVault reproduz a definição de container guardada contra a Docker API, para que o container reapareça no separador Docker do Unraid como se sempre lá tivesse estado (mesma imagem, mesmas definições, mesmos mapeamentos de portas). As VMs têm o seu XML redefinido por SSH e os seus discos e a NVRAM UEFI reanexados, mesmo depois de a VM ter sido eliminada.

Uma só janela restaura todos os tipos. Por predefinição escolhe **Ao lado do original**, que traz o backup de volta como cópia e deixa o original como está. **Substituir o original** faz primeiro backup do estado atual, para que um restauro possa ser desfeito. Uma pasta ou um novo conjunto de dados ZFS são os outros destinos.

Quando um backup para containers dependentes, estes voltam na ordem correta: o BombVault reinicia-os pela ordem `depends_on` do Compose e espera que cada um reporte saudável antes de iniciar os que dependem dele, para que nada avance à frente de uma base de dados ou de um gateway que ainda não esteja disponível. Consulta [Funcionalidades](features.md).

## Como funciona

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

O BombVault usa o socket Docker para parar os containers antes de um backup e recriá-los depois de um restauro. Para as VMs, corre `virsh` no host por SSH (`qemu+ssh://`) para encerrar uma VM de forma ordenada ou tirar um instantâneo a quente. Nunca monta um caminho de libvirt, por isso não pode interferir com o VM Manager do host.

O BombVault é a camada de orquestração e de interface, não o motor de armazenamento. Todo o movimento de dados real passa pelo restic.

## Início rápido

Novo por aqui? Vai a **[Introdução](getting-started.md)** para instalar o BombVault no Unraid através das Community Applications e correr o teu primeiro backup. Depois explora as **[Funcionalidades](features.md)** completas, afina a tua **[Configuração](configuration.md)** e configura o **[Externo e recuperação](offsite-recovery.md)**.

Cada secção pode copiar para vários destinos ao mesmo tempo, um BombVault emparelhado pode guardar as tuas cópias externas num servidor que recusa eliminações, e o **Assistente de emergência** recupera um servidor perdido a partir do kit de recuperação, através do teu grupo ou com a chave escrita à mão. Para levar a configuração para uma máquina nova, usa o cartão **Exportar / importar configurações**. Consulta [Externo e recuperação](offsite-recovery.md) e [Configuração](configuration.md#portable-settings-export-and-import).

A **[Aplicação Android](android.md)** leva todos os servidores do teu grupo para o telemóvel, com o registo de atividade de todos eles num só ecrã.

## Créditos {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** de [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) deu ao BombVault a sua ideia de partida: backup com um clique e reinstalação automática de containers Docker. O BombVault é uma implementação separada, em Go e restic, que leva a ideia às VMs, ao flash e a mais.
- **[restic](https://restic.net/)** é o motor de backup rápido, seguro e com deduplicação que o BombVault comanda.
- **[rclone](https://rclone.org/)** fornece os backends de nuvem.
- A maioria dos glifos dos botões vem do conjunto gratuito Core Solid da **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [fonte](https://github.com/webalys-hq/streamline-vectors)). Os restantes vêm do Font Awesome Free, Material Design Icons, Simple Icons e Tabler Icons, ou foram desenhados para o projeto.

## Licença {#license}

Copyright (C) 2026 Junker der Provinz. O BombVault é software livre sob a **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Pode executá-lo, estudá-lo, partilhá-lo e alterá-lo. Se o distribuir, ou se executar uma versão alterada como serviço de rede, tem de publicar o seu código-fonte sob a mesma licença e manter os avisos de copyright e de atribuição existentes.

O nome e a marca não estão licenciados. A AGPL cobre apenas o código-fonte: "BombVault", o seu logótipo e a sua identidade visual continuam reservados, por isso um fork tem de usar um nome e uma identidade visual próprios e não pode apresentar-se como BombVault.

## Ligações

- **Código-fonte:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Thread de suporte do Unraid:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Issues:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Controlo do host equivalente a root"
    Através do socket Docker, o BombVault pode parar, remover e recriar containers e ler/escrever appdata, e para o backup de VMs inicia sessão no host por SSH para correr `virsh`. Qualquer pessoa que consiga alcançar a interface web tem, efetivamente, root no host. Corre o BombVault apenas numa rede de confiança e não exposta, e define uma palavra-passe de início de sessão em **Definições, Segurança**. O kit de recuperação, os depósitos de chaves e a eliminação dos backups num destino exigem-na. Consulta [Configuração](configuration.md#security-model) para o modelo de segurança completo.
