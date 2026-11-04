# Exemplos práticos

Cada um destes percorre uma tarefa real do princípio ao fim, com nomes e números preenchidos. Apoiam-se na página que explica a funcionalidade, por isso os pormenores estão mesmo ao lado.

## Introdução {#start}

- [O teu primeiro backup de containers, de ponta a ponta](getting-started.md#example-first-backup): da instalação ao kit de recuperação.

## Armazenamento e cópias externas {#storage}

- [Uma cópia externa no Backblaze B2](offsite-recovery.md#example-s3): um bucket, uma chave para esse bucket e a primeira cópia. O Cloudflare R2 funciona da mesma maneira.
- [Um rest-server que geres tu](offsite-recovery.md#example-rest-server): uma cópia externa que recusa eliminações, com a linha `docker run` e o teste de adulteração.
- [O que guarda o Equilibrado](offsite-recovery.md#example-retention): a retenção ao longo de 400 dias de backups noturnos, backup a backup.
- [Os containers mudam para um NAS](offsite-recovery.md#example-own-folder): a pasta própria de uma secção muda de sítio, e o que acontece aos backups já feitos.

## Backups e agendamentos {#backups}

- [Três planos que se encaixam](schedules.md#example): um backup total à noite, as VMs uma vez por semana e um plano próprio para uma entrada a cada 6 horas.
- [O Plex espera enquanto alguém está a ver](backups.md#example-plex): um backup agendado que não interrompe um filme.
- [Uma VM com dois instantâneos do Unraid](vm-backup-ssh-setup.md#example-unraid-snapshots): um backup a quente que mantém os instantâneos, e uma restauração ao lado do original.
- [Uma réplica para uma segunda caixa Unraid](zfs-datasets.md#example-replica): uma cópia ZFS que podes ler a qualquer altura, e três formas de tirar um ficheiro dela.

## Recuperar o que se perdeu {#restore}

- [Recuperar um ficheiro](restore.md#example-single-file): um único ficheiro do backup de containers da noite passada.
- [Um ficheiro de um backup ZFS](zfs-datasets.md#example-zfs-file): o mesmo para um conjunto de dados.
- [Um servidor novo a partir do kit de recuperação](offsite-recovery.md#example-emergency): um servidor morto volta numa máquina nova, sem a sua antiga `APP_KEY` no template.

## Manter-te informado {#staying-informed}

- [O Gotify no telemóvel](notifications.md#example-gotify): as falhas tocam e os sucessos chegam em silêncio.
- [O ntfy como webhook](notifications.md#example-ntfy): mensagens sem conta.

## Vários servidores {#several-servers}

- [Emparelhar duas caixas Unraid](offsite-recovery.md#worked-example-two-unraid-boxes-end-to-end): dois servidores que guardam as cópias um do outro, e nenhum pode eliminar os backups do outro.
- [Um cartão e um alerta no Home Assistant](api.md#home-assistant-example): o BombVault num painel, com uma mensagem quando um backup falha.
