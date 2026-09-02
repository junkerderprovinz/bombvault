# Introdução

Esta página acompanha-o desde uma máquina Unraid acabada de instalar até ao seu primeiro backup.

## Requisitos

| Requisito | Notas |
|---|---|
| **Unraid 6.12+** | Versões anteriores não são testadas. |
| **Localização do repo restic** | Um caminho local (recomendado: o seu array ou cache), SMB, NFS, ou qualquer backend rclone. |
| **Socket Docker** | Montado automaticamente pelo template (`/var/run/docker.sock`). |
| **Flash do Unraid** (`/boot`) | Montada por inteiro pelo template automaticamente (`/boot` para `/host/boot`). Alimenta o backup do flash e permite que um container restaurado reapareça como uma aplicação Unraid normal e editável. |
| **VMs KVM** (opcional) | O backup de VMs comunica com o libvirt por SSH, sem montagem de libvirt. Configure-o em Definições (consulte [Configuração](configuration.md)). |

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

## Anfitrião Docker genérico

Não estás no Unraid? O BombVault também corre como contentor simples em qualquer anfitrião Docker (é também o que sustenta o suporte a contentores no TrueNAS Scale, antes de ter uma entrada própria no catálogo de aplicações).

1. Vai buscar ao repositório o ficheiro [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml), pronto a editar.
2. Define o `APP_KEY` (ver abaixo) e aponta o volume Host Data para a tua raiz de dados real: os comentários do ficheiro explicam as duas coisas.
3. `docker compose up -d` e depois abre `https://<ip-do-anfitrião>:3443/`.

O que muda face ao Unraid:

- **Não há domínio flash/USB.** Não existe pen de arranque para capturar ou repor, por isso o domínio Flash nas definições não tem aqui nada que fazer. Em vez disso, o domínio Ficheiros oferece a sugestão de um clique **Adicionar predefinição: configuração do sistema anfitrião** (um conjunto inicial de ficheiros `/etc` que revês e editas antes de guardar), como equivalente genérico prático.
- **Não há notificações nativas do Unraid.** Os canais de notificação próprios do BombVault (webhook, alertas de falha fora do local, etc.) funcionam normalmente; só é omitido o envio específico para o sistema de notificações do Unraid, já que aqui esse sistema não existe.
- **A cópia de máquinas virtuais é opcional e precisa de um anfitrião libvirtd separado, alcançável por SSH.** Vê o bloco comentado no ficheiro compose. Um anfitrião Docker genérico não traz gestor de máquinas virtuais.

## A única definição obrigatória

A única variável que tem de definir é `APP_KEY`, um segredo hexadecimal de 32 bytes (64 caracteres hexadecimais) usado para derivar a palavra-passe do repositório restic.

Gere um em qualquer máquina:

```bash
openssl rand -hex 32
```

Cole o resultado no campo `APP_KEY` do template.

!!! danger "Não perca a sua APP_KEY"
    Perder a `APP_KEY` torna os seus backups encriptados irrecuperáveis. Guarde-a num local seguro e separado do servidor. Assim que o BombVault estiver a correr, use o seu **kit de recuperação da chave de encriptação** com um clique (consulte [Externo e recuperação](offsite-recovery.md)) para guardar o pacote de recuperação completo.

O template também monta o socket Docker, o flash (`/boot`) e a raiz **Host Data** (`/mnt`) por si. As *origens* e os *destinos* de backup vivem ambos sob Host Data. Para a referência completa de variáveis e a configuração do externo, consulte [Configuração](configuration.md).

## Primeira execução

![O painel após a primeira cópia: o que está protegido, o que corre a seguir e um registo ao vivo.](assets/screenshots/dashboard.png)

*O painel após a primeira cópia: o que está protegido, o que corre a seguir e um registo ao vivo.*

1. Abra a interface web em `https://<your-unraid-ip>:3443` (certificado autoassinado logo de início).
2. Em **Definições**, ative os domínios de backup que pretende (Containers, VMs, Flash, Config, Files) e escolha uma cor de destaque.
3. No separador **Containers**, escolha um container e clique em **Fazer backup** para criar o seu primeiro ponto de restauro. Os caminhos do repositório assumem por predefinição `/mnt/user/bombvault/{container,vms,flash,config,files}` e são criados no primeiro backup.
4. Configure o agendamento em **Definições, Agendamentos**. Existe um *incluir tudo no agendamento* com um clique para containers e VMs.

!!! tip "Opcional: escolha uma ordem de backup"
    Se alguns containers devem ser sempre copiados antes de outros (por exemplo, uma base de dados antes da aplicação que a usa), abra o painel **ordem de backup** na página Containers e arraste-os para a sequência que quiser. As execuções agendadas e de seleção múltipla passam a segui-la; tudo o que deixar sem ordem é copiado pelo mais-em-atraso-primeiro, como antes.

!!! note "Verificação de integração com o host"
    Abra `/spike` na interface web depois de o container arrancar. Sonda cada montagem e CLI (socket Docker, libvirt, restic, qemu-img, rclone) e reporta quaisquer peças em falta, para que possa confirmar que o container está corretamente ligado antes de depender dele.

## Simples vs Avançado

![As definições não têm botão Guardar: cada alteração é escrita no momento.](assets/screenshots/settings.png)

*As definições não têm botão Guardar: cada alteração é escrita no momento.*

Por predefinição, a interface mostra apenas o essencial (fazer backup, restaurar, agendar). Use o interruptor **Simples / Avançado** na barra lateral para revelar os controlos de especialista: retenção, cópia externa, hooks pré/pós, restauro ao nível do ficheiro, notificações, métricas Prometheus e as ferramentas de integridade/manutenção. É uma preferência por navegador e está desligada por predefinição, para que os recém-chegados tenham uma interface limpa e os utilizadores avançados tenham tudo.

## Passos seguintes

- Percorra as **[Funcionalidades](features.md)** completas.
- Adicione uma ou mais réplicas **[Externo e recuperação](offsite-recovery.md)** (cada domínio pode enviar para vários destinos de uma só vez) e guarde o seu kit de recuperação.
- A clonar uma configuração ou a mudar para uma máquina nova? Leve toda a sua configuração consigo com o cartão **Exportar e importar definições**. Consulte [Configuração](configuration.md#portable-settings-export-and-import).
- Encontrou um obstáculo? Consulte **[Resolução de problemas](troubleshooting.md)**.
