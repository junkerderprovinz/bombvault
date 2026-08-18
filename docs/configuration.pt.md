# Configuração

Esta página cobre as variáveis de ambiente do container, as montagens que o template fornece, o backup de VMs por SSH e a configuração do externo. Os **caminhos de repositório** de backup são configurados dentro da aplicação (Definições, Caminhos de backup), não através de variáveis de ambiente.

## Variáveis de ambiente

| Variável | Obrigatória | Descrição |
|---|---|---|
| `APP_KEY` | **Sim** | Segredo hexadecimal de 32 bytes (64 caracteres hex) usado para derivar a palavra-passe do repo restic. Gere com `openssl rand -hex 32`. Guarde-a em segurança: perdê-la torna os backups encriptados irrecuperáveis. |
| `LIBVIRT_HOST` | Para VMs | Host do Unraid alcançado por SSH para o backup de VMs (predefinição `host.docker.internal`; o template pré-preenche um placeholder de IP LAN). Use o IP LAN do seu Unraid, obrigatório numa rede `br0.x` personalizada. |
| `LIBVIRT_SSH_PORT` | Não | Porta SSH do host para o backup de VMs (predefinição `22`). |
| `LIBVIRT_SSH_USER` | Não | Utilizador SSH no host para o backup de VMs (predefinição `root`). |
| `LIBVIRT_URI` | Não | URI de ligação libvirt completo, usado **textualmente** em vez de o construir a partir das três variáveis `LIBVIRT_*` acima (que são então ignoradas para a string de ligação). Não definido por predefinição. Necessário no TrueNAS Scale, cujo libvirtd escuta num socket não padrão que a forma construída não consegue exprimir: `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`. Consulte a secção do TrueNAS Scale em [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md). |
| `PORT` | Não | Porta HTTP (predefinição `3000`; usada apenas com `HTTP_ONLY=true`). |
| `HTTPS_PORT` | Não | Porta HTTPS (predefinição `3443`; o template publica-a 1:1, por isso a WebUI responde em `https://<ip>:3443`). |
| `HTTP_ONLY` | Não | Defina `true` para desativar o listener HTTPS autoassinado e servir apenas HTTP simples (para uso por trás de um proxy reverso que termina o TLS). |
| `HOST_SOURCE_ROOT` | Não | O caminho do host montado como **Host Data** (predefinição `/mnt`). O BombVault traduz as origens de bind-mount que o Docker reporta em caminhos sob esta montagem. Altere apenas se montou uma raiz de host diferente. |
| `DATA_ROOT_SEGMENTS` | Não | Nomes de segmentos de caminho, separados por vírgula, que marcam uma origem de bind-mount como dados de backup (predefinição `appdata`, seguindo a convenção do Unraid `/mnt/user/appdata/<container>`). O bind-mount de um container é automaticamente selecionado para backup quando QUALQUER segmento listado aparece como um segmento de caminho completo da sua origem no host, por exemplo `DATA_ROOT_SEGMENTS=appdata,config` também apanha um bind `.../config`. Consulte [Deteção da origem de backup](#backup-source-detection) para as outras formas, sempre ativas, de encontrar a pasta de dados de um container. |
| `PLATFORM` | Não | Força a plataforma que o BombVault assume estar a correr, em vez de a detetar automaticamente: `unraid`, `generic` ou `truenas` (não definido por predefinição, deteta automaticamente o Unraid ao procurar o seu marcador `dockerMan` sob a montagem flash, caso contrário `generic`; um valor não reconhecido também recai em `generic`, registado no log). Defina-o explicitamente num host Docker genérico ou no TrueNAS Scale em vez de depender da autodeteção exclusiva do Unraid, o ficheiro compose genérico já faz isto. Altera a convenção de recurso de appdata, as predefinições de destino de restauro entre instâncias, e se os passos de notificação/plugin complementar exclusivos do Unraid sequer são tentados (ver `internal/platform`). |
| `BOMBVAULT_SELF_CONTAINER` | Não | O nome do próprio container BombVault, para que nunca faça backup (e assim pare) de si próprio (predefinição `BombVault`; detetado automaticamente através do hostname em rede bridge). |
| `BACKUP_MAX_HOURS` | Não | Máximo de horas de relógio que uma única execução de backup pode reter o bloqueio do seu domínio antes de ser forçada a cancelar (uma salvaguarda para que uma execução encravada não possa bloquear o domínio para sempre). Vazio (a predefinição) usa `48`. Aumente-o para backups em nuvem muito grandes ou lentos (uma execução cancelada no limite falha com `context deadline exceeded`). Defina `0` para desativar o limite por completo. |
| `TZ` | Não | Fuso horário para o agendador (por exemplo `Europe/Berlin`). |

## Montagens

Monte o socket Docker, o flash (`/boot`) e a raiz **Host Data** (`/mnt`) como mostrado no template CA. As *origens* e os *destinos* de backup vivem ambos sob Host Data, e é montada em **rslave** para que uma partilha remota que monte depois de o container arrancar (por exemplo sob `/mnt/remotes`) fique visível sem um reinício.

Os caminhos de repositório de backup assumem por predefinição `/mnt/user/bombvault/{container,vms,flash,config,files}`, criados no primeiro backup. Altere a localização a qualquer momento em **Definições, Caminhos de backup**.

!!! note "Verificação de integração com o host"
    Abra `/spike` na interface web depois de o container arrancar. Sonda cada montagem e CLI (socket Docker, libvirt, restic, qemu-img, rclone) e reporta quaisquer peças em falta.

## Modelo de segurança

!!! warning "Controlo do host equivalente a root"
    Através do socket Docker, o BombVault pode parar, remover e recriar containers e ler/escrever appdata, e para o backup de VMs inicia sessão no host por SSH (`qemu+ssh://`, root por predefinição) para correr `virsh`. Qualquer pessoa que consiga alcançar a sua interface web tem, efetivamente, root no host.

- **Proteção por palavra-passe opcional** (Definições, Segurança): defina uma palavra-passe para exigir início de sessão, limpe-a para desativar. Desligada por predefinição para uso em LAN de confiança. As sessões são assinadas (HMAC derivado da `APP_KEY`) e alterar a palavra-passe invalida-as; os inícios de sessão têm limite de taxa.
- Como o controlo é opcional, quando não está definido, toda a interface e API (incluindo a configuração do externo, as rotas de teste de adulteração e o kit de recuperação) ficam acessíveis a qualquer pessoa que consiga alcançar a porta. Ative o controlo assim que estiver a usar externo, backups imutáveis ou encriptação.
- Corra o BombVault apenas numa rede de confiança e não exposta. Para acesso remoto, coloque-o por trás de um proxy reverso que adicione autenticação e TLS. As respostas transportam cabeçalhos de segurança de base (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- Com `HTTP_ONLY=true`, o cookie de sessão perde a sua flag `Secure` (tem de perder, para funcionar sobre HTTP simples), por isso ative a palavra-passe por trás de um proxy que termina o TLS apenas se a confidencialidade importar.
- A ligação SSH do backup de VMs confia na chave do host no primeiro contacto (TOFU) e fixa-a a partir daí. Verifique a chave do host fora de banda se o seu caminho container-para-host não for de confiança.
- Os backups são encriptados pelo restic quando a encriptação está ativada (Definições; ligada por predefinição), com a chave derivada da `APP_KEY`.

## Backup de VMs por SSH

O BombVault faz backup de VMs KVM/libvirt **sem montar qualquer caminho de libvirt**. Corre `virsh` no host por SSH (`qemu+ssh://`), por isso nunca pode afetar o VM Manager do seu host.

Configuração rápida:

1. **Definições, Sistema, Backup de VM por SSH:** copie a chave pública mostrada.
2. Adicione-a ao `/root/.ssh/authorized_keys` do Unraid (também persistida no flash para sobreviver a reinícios).
3. Clique em **Testar ligação**.

O template adiciona `--add-host=host.docker.internal:host-gateway` para que o container possa alcançar o host. Defina `LIBVIRT_HOST` para o IP LAN do seu Unraid se esse nome não resolver (por exemplo quando o container corre numa rede `br0.x` personalizada). Se alterou a porta SSH do Unraid, defina `LIBVIRT_SSH_PORT` para corresponder. **Os instantâneos a quente** precisam adicionalmente do agente convidado qemu na VM e do disco em `/mnt/cache` (não `/mnt/user`).

!!! important "Guia completo de configuração e rede de VMs"
    O guia completo passo a passo (ativação de SSH, autorização persistente da chave, encaminhamento de rede personalizada e VLAN, método por VM e resolução de problemas no lado do host) está em [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) no GitHub.

## Configuração do externo

Configure uma réplica externa no separador **Definições, Externo**. Consulte [Externo e recuperação](offsite-recovery.md) para o fluxo de trabalho completo (imutável/append-only, teste de adulteração e ensaios de DR). Em resumo:

- **Backends:** SMB/CIFS e NFS (monte a partilha e aponte-lhe um Caminho de backup), backends restic nativos sem rclone (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:user@host:/repo`), ou qualquer remoto rclone (`rclone:<remote>:<bucket>/path`).
- **As credenciais de nuvem** são guardadas encriptadas em Definições, Externo, Credenciais da nuvem.
- **Os destinos SSH não precisam de nada instalado do outro lado.** O `sftp:` só precisa de um servidor SSH. Adicione a chave pública de **Definições, Sistema, Backup de VM por SSH** (também em `/config/ssh/id_ed25519.pub`) ao `~/.ssh/authorized_keys` do utilizador de destino.
- **Cópia externa:** o BombVault replica novos instantâneos com `restic copy` numa base de melhor esforço. O repo local mantém-se primário. Cada domínio tem o seu próprio agendamento externo, mais um botão **Replicar agora**.
- **Vários destinos externos por domínio:** cada domínio pode replicar para vários destinos externos de uma só vez. Adicione destinos extra em Definições, Externo, cada um com o seu próprio repositório, classe de armazenamento S3, flag append-only, retenção e orçamento de crescimento; todos replicam no agendamento externo desse domínio. Uma configuração externa única existente é transferida como o primeiro destino.
- **Retenção por origem:** a política local vive em Definições, Caminhos e Armazenamento; a política externa em Definições, Externo (deixe-a toda a zero para nunca aparar automaticamente os instantâneos externos).
- **Limites de largura de banda:** limite a taxa de envio/receção do restic em Definições, Externo.
- **Classe de armazenamento fria e de arquivo (S3):** para um repo externo S3 nativo, escolha um nível legível para restauro (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). Os remotos rclone definem a sua classe na configuração do rclone.

## Definições portáteis (exportar e importar) {#portable-settings-export-and-import}

O cartão **Exportar e importar definições** na página Definições escreve toda a sua configuração BombVault (definições de domínio, destinos externos, agendamentos, retenção, notificações) para um ficheiro JSON portátil que pode importar noutra instância, para que mudar para uma máquina nova ou clonar uma configuração não signifique reintroduzir tudo à mão. A importação mostra uma pré-visualização e pede confirmação, e nunca toca nos seus dados ou histórico de backup.

!!! warning "A exportação pode conter credenciais"
    Escolhe se inclui as credenciais externas e de notificação no ficheiro. Com as credenciais incluídas, a exportação é tão sensível como o seu kit de recuperação, por isso guarde-a num local seguro. Sem elas, o ficheiro contém apenas definições não secretas.
