# Funcionalidades

Um catálogo curto do que o BombVault faz. Cada linha liga à página que a explica.

## Âmbito do backup {#backup-scope}

- **Containers**: a pasta appdata e a definição do container, com uma escolha de pastas por container. [Backups e entradas](backups.md#what-is-saved)
- **VMs**: imagens de disco, XML e NVRAM UEFI por SSH, com um encerramento ordenado, um instantâneo a quente ou só os blocos alterados. As VMs com instantâneos do Unraid e discos em zvols ficam totalmente cobertas. [Backups de VMs](vm-backup-ssh-setup.md)
- **Flash**: toda a flash USB do Unraid, com uma exportação zip agendada. [Backups e entradas](backups.md#settings-tab)
- **Auto-backup**: as definições e as credenciais do próprio BombVault. [Assistente de emergência](offsite-recovery.md#emergency)
- **Pastas**: qualquer pasta do servidor como conjunto de pastas. [Adicionar uma pasta](backups.md#add)
- **Conjuntos de dados ZFS**: uma árvore de conjuntos de dados lida de um só instantâneo, com ficheiros individuais restauráveis e uma réplica opcional num segundo servidor ZFS. [Conjuntos de dados ZFS](zfs-datasets.md)
- **Despejos da base de dados**: PostgreSQL, MySQL e MariaDB, despejados do servidor em execução para um ponto de restauro próprio. [Despejos da base de dados](backups.md#database-dumps)

## Backups e páginas de entrada {#backups}

- **Uma só lista** com mosaicos, **Filtros**, **Ordenar** e **Pesquisar**, e as últimas catorze execuções de cada linha numa faixa. [A lista](backups.md#list)
- **Seleciona como num gestor de ficheiros**, e faz backup de uma seleção ou pausa-a no servidor. [Escolher várias entradas](backups.md#select)
- **Uma página por entrada** com **Backups**, **Definições** e **Verificação**. [A página de uma entrada](backups.md#entry-page)
- **Etiquetas e comparação** para instantâneos, e **Exportação** como tar, zip ou um repositório restic. [Separador Backups](backups.md#backups-tab)
- **Padrões de exclusão** com pré-visualização em direto, e exclusões recomendadas para aplicações conhecidas. [Padrões de exclusão](backups.md#excludes)
- **Comandos antes e depois**, e **Parar outros contentores** com um reinício controlado pela saúde, por ordem de dependências. [Comandos e outros containers](backups.md#commands)
- **Quando a app está ocupada**: um backup agendado espera pelo Plex, Jellyfin, Emby, por uma verificação tua ou por tráfego calmo, e tenta de novo sozinho. [Quando a aplicação está ocupada](backups.md#app-busy)
- **Instantâneo em vez de parar**: um container em ZFS fica em baixo só alguns segundos. [Agendamentos](schedules.md#snapshot-instead-of-stop)
- **Exportações simples**, opcionalmente seladas com age. [Exportações simples](backups.md#exports)

## Restauração {#restore}

- **Uma página e uma janela** para todos os tipos, com o backup recomendado pré-selecionado. [Restauração](restore.md)
- **Ao lado do original** por predefinição, e **Substituir o original** faz primeiro backup do estado atual. [A janela de restauração](restore.md#window)
- **Ficheiros individuais** para uma pasta ou de volta ao seu lugar. [Recuperar um ficheiro](restore.md#example-single-file)
- **Várias entradas de uma vez**, cada uma a partir do seu backup recomendado ou de um momento. [Restaurar várias entradas](restore.md#several)
- **Stacks Compose inteiras**, iniciadas por ordem de dependências. [Restaurar uma stack](restore.md#stacks)
- **Containers e VMs são recriados** na imagem em que corriam, com GPU, limites e ligações. [O que volta](restore.md#kinds)
- **Verificações antes da restauração**, e uma lista do que ela muda. [A janela de restauração](restore.md#window)
- **Flash como ZIP**, e um plugin de cada vez de volta à flash em uso. [O que volta](restore.md#kinds)
- **ZFS** para dentro do conjunto de dados com um instantâneo de segurança, para uma pasta ou para um novo conjunto de dados com as suas propriedades. [Conjuntos de dados ZFS](zfs-datasets.md#restore)
- **Assistente de emergência** para um servidor perdido, com o kit de recuperação, através do grupo ou com a chave escrita à mão. [Assistente de emergência](offsite-recovery.md#emergency)
- **De uma pasta de backups antiga**, e **Importar do plugin Appdata.Backup**. [Outras origens](restore.md#other-sources)

## Armazenamento e agendamento {#storage-scheduling}

- **Destinos** de todos os tipos: pastas locais, SMB, WebDAV, SFTP, rest-server, S3, discos na nuvem e qualquer remote do rclone. [Destinos](offsite-recovery.md#destinations)
- **Credenciais**: uma só lista com uma predefinida. [Credenciais](offsite-recovery.md#sign-ins)
- **Pastas próprias** neste servidor ou num destino. [A pasta própria de uma secção](offsite-recovery.md#own-folder)
- **O 3-2-1 como fluxo**, em que cada linha é um interruptor, e o **localização por entrada**. [Localização por entrada](offsite-recovery.md#placement)
- **Cópias externas** para vários destinos por secção, com limites de largura de banda, compressão, classes de armazenamento frio e abrandamento enquanto o Plex transmite. [Cópias externas](offsite-recovery.md#off-site-copies)
- **Repositórios diretos**, listados em **Guarda diretamente aqui**. [Repositórios diretos](offsite-recovery.md#direct-repositories)
- **A retenção como linha cronológica**: **Curto**, **Equilibrado**, **Longo** ou **Valores próprios** por destino, com regras por secção. [Durante quanto tempo guardar](offsite-recovery.md#keep)
- **Mudar o historial de uma secção**, e **Remover destino** com ou sem os seus backups. [Mover e remover](offsite-recovery.md#move-remove)
- **Agendamentos que se leem como frases**, a faixa **Semana**, um backup total, planos por secção e um plano próprio por entrada. [Agendamentos](schedules.md)
- **Recuperar backups perdidos**, uma ordem manual e atualizações depois de um backup bem-sucedido. [Agendamentos](schedules.md)

## Visão, verificação e monitorização {#insight-verification-monitoring}

- **A visão geral**: cabeçalho de estado, **Precisa de atenção**, **Estado de proteção**, previsão de armazenamento, histórico e cartões opcionais. [Visão geral](overview.md)
- **Deteção de anomalias** por entrada, que também mantém os backups antigos quando uma origem colapsa. [Anomalias](anomalies.md)
- **Verificação de restauro** depois do primeiro backup, e **Teste de arranque** numa cópia isolada. [Separador Verificação](backups.md#verify-tab)
- **Verificações agendadas e restauros de teste** a partir da cópia externa. [Testes DR](offsite-recovery.md#dr-drills)
- **Tamanho por pasta**, e porque é que um backup foi lento. [O que uma execução te diz](backups.md#run-notes)
- **Notificações**: webhook, Gotify, Matrix, e-mail, Apprise, Unraid e Healthchecks, com um resumo semanal e um relatório de atrasos. [Notificações](notifications.md)
- **Prometheus**, um dispositivo Home Assistant, uma API HTTP e mDNS. [API e integrações](api.md)
- **Relatório de erro**: um ZIP com dados ocultados para um issue do GitHub. [Resolução de problemas](troubleshooting.md#still-stuck)

## Proteção contra ransomware {#ransomware-protection}

- **Proteção contra eliminação (append-only)**, imposta pelo outro lado e provada por um teste de adulteração. [Proteção contra eliminação](offsite-recovery.md#delete-protection-append-only)
- **Aceitar sem proteção contra eliminação** para um lugar que não consegue recusar uma eliminação. [Proteção contra eliminação](offsite-recovery.md#delete-protection-append-only)
- **Eliminar backups de vez pede o teu próprio início de sessão** quando há uma palavra-passe de acesso, e um telemóvel emparelhado não o pode fazer. [Modelo de segurança](configuration.md#security-model)
- **Kit de recuperação**, opcionalmente selado com age, e um **guardião da chave** no teu grupo. [Kit de recuperação](offsite-recovery.md#recovery-kit), [Guardião da chave](offsite-recovery.md#key-keeper)

## Vários servidores {#several-servers}

- **Emparelhamento por doze palavras**, encriptado de ponta a ponta, diretamente ou através de um relay. [Emparelhar instâncias](offsite-recovery.md#pairing)
- **A página Instâncias** com o grupo como uma rede e as funções **Recetor**, **Recolhedor**, **Servidor ZFS** e **Guardião da chave**. [Instâncias e funções](offsite-recovery.md#roles)
- **Um servidor recetor** configurado por ti, em modo append-only. [Servidor recetor](offsite-recovery.md#receiving-server)

## Assistentes de IA (MCP) {#mcp}

Um servidor MCP incorporado deixa um assistente ler o estado dos backups, a cobertura, o historial de execuções e os pontos de restauro, e com uma chave que o permita iniciar um backup. As restaurações, as eliminações, a poda e as definições ficam na interface web. [Servidor MCP](mcp.md)

## Aplicações e companheiros {#apps}

- **App para Android**: todos os servidores do teu grupo no telemóvel. [Aplicação Android](android.md)
- **BombVault Widget**: um mosaico no painel do Unraid. [Aplicações](settings.md#apps)
- **Registo de atividade incorporável** para painéis como o Homepage ou o Heimdall. [Widget de painel](api.md#dashboard-widget)

## Outros {#other}

- **42 idiomas**, três temas, e rótulos, cores, cantos e animação à tua escolha. [Aspeto](settings.md#look)
- **Definições que se guardam sozinhas**, com uma pesquisa em todos os catorze mosaicos. [Definições](settings.md)
- **Novidades** depois de uma atualização, com pontos verdes no que mudou. [Info](settings.md#info)
- **HTTPS logo de origem**, e uma verificação de saúde para ferramentas de autorrecuperação. [Configuração](configuration.md#network)
