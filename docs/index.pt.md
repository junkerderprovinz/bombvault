# BombVault

**Os teus dados do Unraid, selados num cofre. Larga um backup. Detona um restauro.**

![A visão geral: o cabeçalho de estado, o que precisa de ti, a proteção de cada secção, os destinos de armazenamento e o histórico.](assets/screenshots/overview/home-dark.png#only-dark)
![A visão geral: o cabeçalho de estado, o que precisa de ti, a proteção de cada secção, os destinos de armazenamento e o histórico.](assets/screenshots/overview/home-light.png#only-light)

O BombVault é uma aplicação web self-hosted para **backup e recuperação total de desastres** no Unraid e noutros anfitriões Docker, como o TrueNAS SCALE, o ZimaOS ou uma simples máquina Linux. Faz backup de containers Docker, VMs KVM/libvirt, pastas, conjuntos de dados ZFS e da unidade flash do Unraid. Corre como um único container Docker multi-arch, dá-te uma interface web num tema escuro, claro ou do sistema e trata de todo o ciclo de vida: fazer backup, agendar, verificar e restaurar.

Os restauros são automáticos. No Unraid, os containers reaparecem no separador Docker exatamente como antes, e as VMs são redefinidas no Gestor de VMs com os seus discos e a NVRAM UEFI voltados a ligar. Sem reinstalações manuais, sem reconfiguração, sem dramas. No TrueNAS, um container restaurado ao lado do original corre como um container Docker simples que o ecrã Apps não lista.

Assente no [restic](https://restic.net), por isso cada backup é deduplicado, incremental e encriptado por predefinição.

!!! note "Guarda a tua APP_KEY em segurança"
    O BombVault deriva a palavra-passe do repositório restic a partir de um segredo de 32 bytes chamado `APP_KEY`. Perdê-lo, juntamente com o kit de recuperação, torna os backups encriptados irrecuperáveis. Gera um com `openssl rand -hex 32` e guarda-o num local seguro. Consulta [Configuração](configuration.md).

## O que o BombVault protege {#what-bombvault-protects}

Cada container, VM, conjunto de pastas, a flash, cada conjunto de dados ZFS e as definições do próprio BombVault são uma **entrada** com uma página própria. As entradas do mesmo tipo formam uma **secção**, que ligas em **Definições, Geral**.

| Secção | O que é guardado |
|---|---|
| **Containers** | Diretório appdata mais a definição do container (imagem, variáveis de ambiente, portas, etiquetas, volumes). |
| **VMs** | Imagens de disco da VM, a definição XML e a NVRAM UEFI, com backup por SSH (sem montagem de libvirt). |
| **Flash** | Toda a flash USB (`/boot`): sistema, licença, configuração do array, partilhas, rede e configuração de plugins. |
| **Auto-backup** | O próprio `/config` do BombVault: a base de dados de definições, as credenciais dos seus destinos e o par de chaves SSH do libvirt. |
| **Pastas** | Qualquer pasta do servidor como conjunto de pastas, cada um com padrões de exclusão opcionais. |
| **Conjuntos de dados ZFS** | Um conjunto de dados com todos os que estão abaixo dele, lido de um instantâneo ZFS e guardado como uma pasta. Vê [Conjuntos de dados ZFS](zfs-datasets.md). |

## O restauro é a estrela {#restore-is-the-star}

Depois de copiar os dados de volta a partir do instantâneo do restic, o BombVault reproduz a definição de container guardada contra a API do Docker, para que o container reapareça no separador Docker do Unraid como se sempre lá tivesse estado (mesma imagem, mesmas definições, mesmos mapeamentos de portas). As VMs têm o seu XML redefinido por SSH e os seus discos e a NVRAM UEFI voltados a ligar, mesmo depois de a VM ter sido eliminada.

Uma só janela restaura todos os tipos. Por predefinição escolhe **Ao lado do original**, que traz o backup de volta como cópia e deixa o original como está. **Substituir o original** faz primeiro backup do estado atual, para que uma restauração possa ser desfeita. Uma pasta ou um novo conjunto de dados ZFS são os outros destinos.

Quando um backup pára containers dependentes, estes voltam na ordem certa: o BombVault reinicia-os pela ordem `depends_on` do Compose e espera que cada um se declare saudável antes de iniciar os que dependem dele, para que nada avance à frente de uma base de dados ou de um gateway que ainda não esteja de pé. Vê [Restauração](restore.md) e [Comandos e outros containers](backups.md#commands).

## Como funciona {#how-it-works}

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + sign-ins (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

O BombVault usa o socket Docker para parar os containers antes de um backup e recriá-los depois de uma restauração. Para as VMs, corre `virsh` no host por SSH (`qemu+ssh://`) para encerrar uma VM de forma ordenada ou tirar um instantâneo a quente. Nunca monta um caminho do libvirt, por isso não pode atrapalhar o Gestor de VMs do host.

O BombVault é a camada de orquestração e de interface, não o motor de armazenamento. Todo o movimento de dados passa pelo restic.

## Início rápido {#quick-start}

Novo por aqui? Vai a **[Introdução](getting-started.md)** para instalares o BombVault e correres o teu primeiro backup, e depois configura o **[Externo e recuperação](offsite-recovery.md)**. Os **[exemplos práticos](guides.md)** percorrem as tarefas mais comuns do princípio ao fim, a página **[Definições](settings.md)** lista todos os mosaicos, e as **[Funcionalidades](features.md)** são o catálogo curto de tudo o que o BombVault faz.

Cada secção pode copiar para vários destinos ao mesmo tempo, um BombVault emparelhado pode guardar as tuas cópias externas num servidor que recusa eliminações, e o **Assistente de emergência** recupera um servidor perdido a partir do kit de recuperação, através do teu grupo ou com a chave escrita à mão. Para levares a configuração para uma máquina nova, usa o cartão **Exportar / importar configurações**. Vê [Externo e recuperação](offsite-recovery.md) e [Configuração](configuration.md#portable-settings-export-and-import).

A **[Aplicação Android](android.md)** leva todos os servidores do teu grupo para o telemóvel, com o registo de atividade de todos eles num só ecrã.

A documentação existe em 26 idiomas e a interface em 42. As páginas que ainda não têm tradução aparecem em inglês.

## Palavras usadas aqui {#glossary}

Algumas palavras têm um significado fixo no BombVault e nestas páginas.

| Palavra | Significado |
|---|---|
| **Entrada** | Uma coisa de que o BombVault faz backup: um container, uma VM, um conjunto de pastas, a unidade flash, um conjunto de dados ZFS com os que estão abaixo dele, ou as suas próprias definições. Cada uma tem uma página própria. |
| **Secção** | Todas as entradas de um tipo, como os Containers. Ligas as secções em **Definições, Geral**. |
| **Destino** | Um lugar para onde vão os backups: uma pasta neste servidor, uma partilha, um servidor teu ou um serviço de nuvem. Vê [Destinos](offsite-recovery.md#destinations). |
| **Local** | O destino neste servidor que guarda as pastas das secções. |
| **Pasta própria** | O repositório para onde uma secção escreve primeiro os seus backups. Por predefinição fica neste servidor e pode estar num destino. Vê [A pasta própria de uma secção](offsite-recovery.md#own-folder). |
| **Cópia** | Um backup copiado da pasta própria para outro destino com `restic copy`. |
| **Repositório direto** | Um repositório num destino externo para onde as entradas fazem backup diretamente, sem uma cópia local pelo meio. Vê [Repositórios diretos](offsite-recovery.md#direct-repositories). |
| **Credencial** | A chave ou palavra-passe guardada para um serviço S3 ou um rest-server. Uma delas é a predefinida. Vê [Credenciais](offsite-recovery.md#sign-ins). |
| **Localização por entrada** | Que destinos recebem os backups de uma entrada, definido na entrada ou como predefinição da sua secção. Vê [Localização por entrada](offsite-recovery.md#placement). |
| **Réplica** | Uma cópia exata ao bloco de uma entrada ZFS num segundo servidor ZFS, feita com `zfs send`. Fica ao lado do backup e não o substitui. Vê [Réplica](zfs-datasets.md#replica). |
| **Função** | Algo que uma instância do teu grupo faz por outra: Recetor, Recolhedor, Servidor ZFS ou Guardião da chave. Vê [Instâncias e funções](offsite-recovery.md#roles). |

## Créditos {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** de [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) deu ao BombVault a sua ideia de partida: backup com um clique e reinstalação automática de containers Docker. O BombVault é uma implementação separada, em Go e restic, que leva a ideia às VMs, ao flash e a mais.
- **[restic](https://restic.net/)** é o motor de backup rápido, seguro e com deduplicação que o BombVault comanda.
- **[rclone](https://rclone.org/)** fornece os backends de nuvem.
- A maioria dos glifos dos botões vem do conjunto gratuito Core Solid da **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [fonte](https://github.com/webalys-hq/streamline-vectors)). Os restantes vêm do Font Awesome Free, Material Design Icons, Simple Icons e Tabler Icons, ou foram desenhados para o projeto.

## Licença {#license}

Copyright (C) 2026 Junker der Provinz. O BombVault é software livre sob a **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Podes executá-lo, estudá-lo, partilhá-lo e alterá-lo. Se o distribuíres, ou se executares uma versão alterada como serviço de rede, tens de publicar o teu código-fonte sob a mesma licença e manter os avisos de copyright e de atribuição existentes.

O nome e a marca não estão licenciados. A AGPL cobre apenas o código-fonte: "BombVault", o seu logótipo e a sua identidade visual continuam reservados, por isso um fork tem de usar um nome e uma identidade visual próprios e não pode apresentar-se como BombVault.

## Ligações {#links}

- **Código-fonte:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Thread de suporte do Unraid:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Issues:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Controlo do host equivalente a root"
    Através do socket Docker, o BombVault pode parar, remover e recriar containers e ler e escrever o appdata, e para o backup de VMs inicia sessão no host por SSH para correr `virsh`. Qualquer pessoa que consiga alcançar a interface web tem, na prática, root no host. Corre o BombVault apenas numa rede de confiança e não exposta, e define uma palavra-passe de acesso em **Definições, Segurança**. O kit de recuperação, os depósitos de chaves e a eliminação dos backups num destino exigem-na. Consulta [Configuração](configuration.md#security-model) para o modelo de segurança completo.
