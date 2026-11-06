# Definições

As **Definições** são catorze mosaicos. Cada mosaico é uma página de cartões, e cada alteração é escrita à medida que a fazes, com um breve clarão no controlo, ou um abanão quando o servidor a recusa. Algumas janelas mantêm um botão **Guardar** porque guardá-las a meio seria arriscado, por exemplo um início de sessão ou a palavra-passe de acesso. As explicações estão nos balões (i) ao lado de cada controlo.

Carrega em `/` ou Ctrl+K em qualquer ponto das definições para pesquisar em todas elas, também pelos nomes que as páginas tinham em versões anteriores. A ordem dos mosaicos pode ser mudada em **Aspeto**. Onde uma coluna de mosaicos ao lado da página a apertaria, como numa janela com cerca de 1200 píxeis de largura ou num telemóvel, os mosaicos passam a uma faixa por cima da página que desliza para os lados.

![As Definições com o mosaico Geral aberto.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![As Definições com o mosaico Geral aberto.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Esta página lista cada mosaico e os seus cartões, e liga à página que explica cada um em profundidade.

## Geral {#general}

- **Secções** liga ou desliga cada secção: **Containers**, **VMs**, **Flash**, **Pastas**, **Conjuntos de dados ZFS** e **Auto-backup**. Uma secção desligada não é copiada por nenhum agendamento, nem pelo plano próprio de uma entrada, e as suas entradas não contam. **Fazer backup agora** continua a funcionar. O **Flash** só aparece no Unraid.
- **Containers** guarda o que se aplica a todos os containers, por exemplo **Despejos automáticos das bases de dados**. Tudo o que afeta um só container está na página dele. Vê [Despejos da base de dados](backups.md#database-dumps).

## Aspeto {#look}

- **Aspeto**: **Idioma** (42 idiomas), **Tema** (**Escuro**, **Claro** ou **Sistema**), **Cantos**, **Animação** (**Desativado**, **Subtil**, **Selvagem** ou **Furioso**), **Notificações discretas**, que esconde avisos de sucesso como as confirmações de gravação, e **Ordem das páginas de definições**, onde seguras um mosaico e o arrastas para o lugar, ou usas Alt e as setas.
- **Rótulos**: quanto de um controlo é mostrado, em separado para **Botões**, **Barra lateral**, **Separadores** e **Barra inferior**: **Texto**, **Texto e símbolo**, **Símbolo**, ou **Reativo**, que mostra só o símbolo e desliza as palavras quando apontas para ele. A **Barra lateral** afeta só a disposição de secretária e a **Barra inferior** só a do telemóvel.
- **Cores**: uma **Cor de destaque**, ou o **Modo arco-íris**, em que cada linha de uma lista tem a sua própria cor de uma paleta de oito. O Modo arco-íris acrescenta o **Modo disco**, em que as cores deslizam devagar para a seguinte, o **Modo reativo**, em que a cor de uma linha só aparece enquanto apontas para ela, ela corre ou está selecionada, a **Rotação de cores** e a própria **Paleta de cores**.

O BombVault guarda o aspeto no servidor. Um navegador que chega pela primeira vez adota-o. Quando o teu sistema pede menos movimento, isso ganha sempre à **Animação**.

## Agendamento {#schedule}

**Semana**, **Backup total** com **Quando os backups correm**, **Recuperar backups perdidos após o arranque** e **Comandos antes e depois**, **Agendamentos por secção** com **Cópias externas por secção**, **Entradas agendadas**, **Ordem** com **Reiniciar após o backup**, **Instantâneo em vez de parar**, **Atualizações após o backup** e **Inatividade antes do backup**. Vê [Agendamentos](schedules.md).

## Destinos {#storage-locations}

- **Mostrar também na barra lateral** fixa a página Destinos na barra lateral.
- A lista dos teus destinos, com **Adicionar destino**. Um clique abre a página de um destino, vê [A página de um destino](offsite-recovery.md#location-page).
- **Credenciais**: os inícios de sessão guardados com a estrela de predefinição, vê [Credenciais](offsite-recovery.md#sign-ins).
- **Destinos por secção**: o fluxo 3-2-1, em que cada linha é um interruptor, vê [Predefinições por secção](offsite-recovery.md#placement-defaults).
- **Repositórios**: repositórios feitos para entradas individuais, vê [Localização por entrada](offsite-recovery.md#placement).
- **Durante quanto tempo guardar** aponta para os destinos, onde se define a retenção, e **Pré-visualizar** mostra **O que a próxima execução iria remover**, por repositório e por entrada, antes de acontecer. Não muda nada e não bloqueia nada, por isso responde mesmo enquanto um backup corre.
- **Cópias externas**: uma linha por secção com **Testar ligação**, **Replicar agora** e **Configurar** ou **Editar**, vê [Cópias externas](offsite-recovery.md#off-site-copies).
- **Largura de banda externa**: **Limite de envio (KiB/s)** e **Limite de receção (KiB/s)** para todas as cópias. 0 significa sem limite.

O cartão **Streaming**, que baixa o limite de envio enquanto o Plex, o Jellyfin ou o Emby transmitem, fica na própria página Destinos.

## Restauro {#restore}

- Uma ligação à página **Restauração**.
- **Pastas de restauro**: a **Pasta de restauro predefinida** para uma restauração para uma pasta, com **Subpasta por secção** e uma **Exceção** por secção. Vê [Para onde vai uma restauração para uma pasta](restore.md#folders).
- **Registries de containers**: inícios de sessão para registries privados, por exemplo uma imagem `ghcr.io` reservada a patrocinadores. Uma restauração usa-os para transferir uma imagem em falta, e uma atualização depois do backup para verificar se há uma mais recente. As imagens públicas são transferidas sem eles. **Adicionar registry** oferece registries públicos, alojados por ti, os de um fornecedor de nuvem e **Outro registry** para qualquer endereço.
- **Para emergências**: **Descarregar kit de recuperação**, que precisa de uma palavra-passe de acesso. Vê [Kit de recuperação](offsite-recovery.md#recovery-kit).

## Notificações {#notifications}

**Notificações** com **Notificar**, os resumos, **Comunicar backups em atraso**, **Notificações do Unraid** e o **Resumo semanal**, **Canais** com Webhook, Apprise, Matrix, Gotify e E-mail (SMTP), e **Healthchecks**. Vê [Notificações](notifications.md).

## Verificação {#verify}

- **Verificar os backups**: o resultado do último `restic check` por secção, com **Desbloquear** e **Podar** em **Mais** em cada linha. **Desbloquear** limpa um bloqueio antigo, e **Podar** aplica as regras de retenção a pedido.
- **Verificações automáticas**: a verificação de restauro agendada com a sua **Amostra de dados (%)** e alvos de teste, o **Teste de arranque**, que também restaura um container numa cópia isolada e o arranca, o **Teste DR externo agendado** e **Verificar proteção contra eliminação**, o teste de adulteração agendado. Vê [Testes DR](offsite-recovery.md#dr-drills) e [Teste de adulteração](offsite-recovery.md#tamper-test).
- **Anomalias**: a deteção, a sensibilidade e que achados enviam uma mensagem. Vê [Anomalias](anomalies.md).

## Segurança {#security}

- **Palavra-passe**: a palavra-passe de acesso, com pelo menos 12 caracteres. Mudá-la ou limpá-la pede a atual e, com a autenticação de dois fatores ligada, o código.
- **Autenticação de dois fatores**: um código de uma aplicação de autenticação por cima da palavra-passe, com oito códigos de recuperação mostrados uma só vez.
- **Chaves de acesso**: inicia sessão com uma chave no telemóvel, no portátil ou numa chave de segurança. Precisam de um nome de domínio verdadeiro e de um certificado em que o navegador confie.
- **App para Android**: **Os telemóveis podem emparelhar com as doze palavras do grupo**, o que deixa um telemóvel emparelhado iniciar sessão sem a palavra-passe.
- **Encriptação**: **Encriptar backups**, ligado por predefinição.

Várias coisas precisam de uma palavra-passe de acesso, como o kit de recuperação e os depósitos de chaves. Vê [Modelo de segurança](configuration.md#security-model).

## Instâncias {#instances}

- **Mostrar também na barra lateral** fixa a página Instâncias na barra lateral.
- **Vários servidores** liga as partes de um grupo: **Instâncias**, a página com um cartão para cada membro, **Recetor** e **Recolha**. Vê [Instâncias e funções](offsite-recovery.md#roles).
- Uma ligação à página Instâncias, com as partes que estão ligadas.

## Emparelhamento {#pairing}

**Emparelhamento** com **Gerar frase** ou **Introduzir frase**, os cartões de passos, o nome desta instância e **Endereços conhecidos**, o cartão **Relay** com as suas quatro formas e **O que o relay vê**, os membros do grupo, e **Servidor ZFS sem BombVault** para um destino de réplica por SSH. Vê [Emparelhar instâncias](offsite-recovery.md#pairing) e [Enviar para um servidor ZFS](zfs-datasets.md#replica-push).

## Integrações {#integrations}

- **Servidor MCP**: chaves para assistentes de IA, **Escolher um cliente** e o registo de cada chave. Vê [Servidor MCP](mcp.md).
- **Tokens da API**: tokens com nome para scripts e painéis. Vê [Tokens](api.md#tokens).
- **Serviços**: **Home Assistant** por MQTT, **Encontrar na rede** (mDNS como `bombvault.local`) e **Expor /metrics** para o Prometheus, com um **Token de scrape (opcional)**. Vê [Home Assistant](api.md#home-assistant) e [Encontrar o BombVault na rede](api.md#mdns).
- **Widget de painel**: um token só de leitura e um endereço para qualquer painel que mostre um iframe, como o Homepage ou o Heimdall, só com o registo de atividade em direto. Vê [Widget de painel](api.md#dashboard-widget).
- **SSH do anfitrião**: a chave pública do BombVault e **Testar ligação**, para os backups de VMs e de conjuntos de dados ZFS e para as notificações do Unraid. Vê o [guia de backup de VMs](vm-backup-ssh-setup.md).

## Aplicações {#apps}

- **App para Android**: o APK da versão que o teu servidor corre, com um código QR, e a entrada no F-Droid. Vê [Aplicação Android](android.md#install).
- **ParleyPort**: o container de relay com o seu template Unraid e o comando Docker que o inicia.
- **BombVault Widget**: um mosaico no painel do Unraid com o registo de atividade do BombVault e a próxima execução. Com uma ligação SSH ao host, o cartão instala e remove o plugin sozinho. Sem ela, dá-te o endereço do `.plg` para instalares em **Plugins, Install Plugin**.

## Sistema {#system}

- **Exportar / importar configurações**: toda a tua configuração como um ficheiro JSON portátil, com ou sem as credenciais. A importação mostra primeiro **Este ficheiro contém** e deixa de fora as ligações a destinos que já não existem. **Cifrar exportações com age** sela as exportações simples, o ZIP do flash e o kit de recuperação para os **Destinatários age** que listares. Vê [Configurações portáteis](configuration.md#portable-settings-export-and-import).
- **Desempenho**: **Threads de CPU (0 = todos)**, **Limite de tamanho da cache (MB, 0 = ilimitado)** para a cache do restic em `/config`, e **Limite de memória por backup (MiB, 0 = nenhum)**, um limite suave que faz um backup grande abrandar em vez de ser morto por falta de memória.

## Info {#info}

- **Versão**: BombVault, GlimStone, restic e rclone, com **Procurar atualizações** e **Novo**, que conta as alterações que ainda não viste e abre uma janela que as lista.
- **Ajuda**: o **Manual** abre esta documentação, e o **Relatório de erro** descarrega um ZIP com dados ocultados para um issue do GitHub. Precisa de uma palavra-passe de acesso.
- **Sobre o BombVault**: o projeto, a sua licença e como contactar o autor.
