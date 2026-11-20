# Nós

Um nó é um BombVault sem interface web, por exemplo num VPS ou num pequeno servidor em casa de um amigo. Faz backup dos containers e das pastas do seu próprio servidor, e o BombVault onde o adicionas, a que esta página chama **instância base**, diz-lhe quando fazer backup e para onde vão os backups. Geres o nó a partir da interface web da instância base, ao lado das tuas próprias entradas.

![O cartão do nó vps-falkenstein na página Instâncias: Ligada, a sua versão, os seus backups no servidor recetor, quando cada secção faz backup, e Alterar destino, Substituir e Libertar.](assets/screenshots/nodes/card-dark.png#only-dark)
![O cartão do nó vps-falkenstein na página Instâncias: Ligada, a sua versão, os seus backups no servidor recetor, quando cada secção faz backup, e Alterar destino, Substituir e Libertar.](assets/screenshots/nodes/card-light.png#only-light)

## Como funciona {#how-it-works}

- **A instância base guarda a chave do nó.** Quando adicionas um nó, a instância base cria a sua `APP_KEY`, guarda uma cópia selada e entrega-ta uma única vez, no ficheiro compose do nó. Com essa cópia, a instância base chega ao nó e abre os seus backups.
- **O nó não precisa de nenhuma porta aberta.** A instância base e o nó falam pelo relay através de uma ligação privada só deles, à parte do teu grupo de emparelhamento. A instância base inicia cada chamada e assina-a, e o nó não lhe pede nada além do seu olá. O único destino que precisa de uma porta aberta é [guardar os backups no nó](#keep-on-node).
- **Os backups nunca passam pelo relay.** O nó envia-os diretamente para o sítio que escolheste: o servidor recetor da instância base, um dos seus destinos ou um rest-server no próprio nó.
- **O nó não planeia nada sozinho.** Não tem agendamentos, não envia notificações e não entra em nenhum grupo nem em nenhuma fortaleza. A instância base inicia os seus backups, acompanha como correm e envia os alertas.
- **Um nó faz backup de containers e conjuntos de pastas.** As VMs, a unidade flash, os conjuntos de dados ZFS e o auto-backup ficam para as instalações completas.

A ligação usa o relay próprio da instância base quando o seu grupo tem um, e caso contrário o relay do projeto, mesmo quando a instância base emparelha sem relay.

## Antes de começar {#before-you-start}

- **Uma palavra-passe de início de sessão na instância base.** Adicionar um nó, mostrar outra vez o seu ficheiro compose e abrir as suas entradas exigem um início de sessão com a palavra-passe, vê [Início de sessão](configuration.md#login).
- **Docker com Compose no servidor do nó.**
- **Um sítio para os backups**, um dos três descritos em [Para onde vão os backups](#placement). O servidor recetor é a escolha mais segura, porque lá o nó não consegue apagar nada.

## Adicionar um nó {#add}

1. Na instância base, abre **Instâncias** e escolhe **Adicionar nó** em baixo à direita.
2. **Nome.** Escreve como o nó vai aparecer aqui, por exemplo o nome do servidor, e escolhe **Seguinte**.

    ![A janela Adicionar nó no primeiro passo, com o nome vps-helsinki escrito.](assets/screenshots/nodes/add-name-dark.png#only-dark)
    ![A janela Adicionar nó no primeiro passo, com o nome vps-helsinki escrito.](assets/screenshots/nodes/add-name-light.png#only-light)

3. **Backups.** Escolhe para onde vão os backups e preenche o que esse sítio pede. **Faz backup todos os dias às** é a hora a que as duas secções começam. A instância base sugere meia hora depois dos seus próprios backups de containers, para que os dois não usem os mesmos discos ao mesmo tempo. Mais tarde podes dar a cada secção o seu próprio plano no cartão. Escolhe **Seguinte**.

    ![O segundo passo: o servidor recetor escolhido, Endereço para os nós preenchido com um endereço https e Faz backup todos os dias às 03:00.](assets/screenshots/nodes/add-placement-dark.png#only-dark)
    ![O segundo passo: o servidor recetor escolhido, Endereço para os nós preenchido com um endereço https e Faz backup todos os dias às 03:00.](assets/screenshots/nodes/add-placement-light.png#only-light)

4. **Arranque.** A janela mostra o ficheiro compose do nó com `MODE`, `APP_KEY`, `NODE_HOME` e `NODE_NAME` já preenchidos. Copia-o e guarda a `APP_KEY` num sítio seguro. No servidor do nó:

    ```sh
    mkdir -p /opt/bombvault-node && cd /opt/bombvault-node
    nano docker-compose.yml   # paste the file from the window
    docker compose up -d
    ```

    Antes de o arrancares, aponta o volume Host Data para os dados de que queres fazer backup e define `HOST_SOURCE_ROOT` com o mesmo caminho, como mostra o exemplo abaixo.

5. A janela espera pelo nó. Assim que o nó responde, a instância base diz-lhe para onde vão os seus backups e verifica que ele lá chega, e a janela diz **vps-helsinki está online e recebe ordens desta instância.**

    ![O último passo: o ficheiro compose e depois o nó online, com os backups no servidor recetor.](assets/screenshots/nodes/add-start-dark.png#only-dark)
    ![O último passo: o ficheiro compose e depois o nó online, com os backups no servidor recetor.](assets/screenshots/nodes/add-start-light.png#only-light)

Podes fechar a janela antes de o nó responder. O cartão diz então **À espera do nó**, e a instância base diz-lhe para onde vão os backups assim que ele aparecer.

### Exemplo: um VPS que faz backup para o servidor recetor {#example-vps}

A instância base é uma máquina Unraid chamada `tower` que corre o [servidor recetor](offsite-recovery.md#receiving-server). Um proxy reverso publica-o como `https://backup.tower.example.org`. O VPS `vps-helsinki` corre o Nextcloud e o Caddy, com os dados em `/srv/appdata`.

1. Em `tower`: **Instâncias**, **Adicionar nó**, nome `vps-helsinki`, **Seguinte**.
2. Escolhe **Servidor recetor desta instância** e escreve `https://backup.tower.example.org` em **Endereço para os nós**. Os containers da instância base fazem backup às 02:30, por isso a janela sugere as 03:00. Deixa ficar e escolhe **Seguinte**.
3. No VPS, guarda o ficheiro da janela com os volumes e o `HOST_SOURCE_ROOT` alterados para `/srv/appdata`:

    ```yaml
    services:
      bombvault-node:
        image: junkerderprovinz/bombvault:latest
        restart: unless-stopped
        hostname: bombvault-node
        container_name: bombvault-node
        volumes:
          - ./config:/config
          - /var/run/docker.sock:/var/run/docker.sock
          - /srv/appdata:/host/user
        environment:
          MODE: node
          APP_KEY: "<filled in by home>"
          NODE_HOME: "<filled in by home>"
          NODE_NAME: "vps-helsinki"
          PLATFORM: generic
          HOST_SOURCE_ROOT: /srv/appdata
    ```

4. `docker compose up -d`. Uns segundos depois, a janela em `tower` diz que o nó está online e **Os backups vão para https://backup.tower.example.org.**
5. Às 03:00 a instância base inicia os backups do nó. Na manhã seguinte, o cartão **Nós** na visão geral mostra o último backup de `vps-helsinki`, e **Backups** lista os seus containers numa secção própria.

A forma como o BombVault encontra os dados de um container num host Docker simples está descrita em [Deteção das fontes de backup](configuration.md#backup-source-detection).

## Para onde vão os backups {#placement}

Cada nó tem um único destino para as suas duas secções. **Alterar destino** no cartão muda-o assim que o nó está online. O sítio novo recebe repositórios novos, e os backups no sítio antigo ficam onde estão.

### Servidor recetor desta instância {#to-receiver}

O nó escreve no servidor recetor da instância base em modo append-only: pode adicionar backups, mas não pode apagar nenhum. Recebe lá um início de sessão próprio, com o nome do nó, e um repositório por secção na sua própria pasta. A instância base junta a sua própria chave a cada repositório e poda-os ela mesma, já que o nó não pode.

O nó chega ao servidor pelo **Endereço para os nós**, que defines no segundo passo ou na janela **Servidor recetor**. Um nó costuma enviar a partir da internet, por isso o endereço tem de ser um destes:

- `https://`, por exemplo atrás de um proxy reverso, do Pangolin ou de um túnel. Quando o servidor mostra um certificado próprio em vez de um de uma autoridade pública, cola-o em formato PEM em **Certificado**.
- `http://` com um endereço IP privado, como `10.0.0.5` numa ligação WireGuard ou `100.101.102.103` no Tailscale.

Um `http://` simples para um endereço público é recusado, porque o nó enviaria os backups pela internet sem encriptação. Quando mudas o endereço, a instância base avisa cada nó do recetor, e um nó que esteja ausente fica a saber assim que voltar.

### Um destino {#to-location}

O nó escreve num dos destinos da instância base a que consegue chegar sozinho: um serviço S3 ou um servidor REST, por `https://` ou num endereço privado. Os seus repositórios ficam lá em `bombvault-nodes/<node id>/containers` e `bombvault-nodes/<node id>/files`.

O nó inicia sessão com umas [credenciais guardadas](offsite-recovery.md#sign-ins) criadas para ele, que têm de ser diferentes das que a instância base usa para escrever nesse destino. Se a janela disser que não há nenhumas, adiciona-as primeiro. Com essas credenciais o nó costuma conseguir apagar o que está no destino, ao contrário do servidor recetor, e a janela avisa disso.

### Guardá-los no nó {#keep-on-node}

O nó guarda ele próprio os seus backups, atrás de um rest-server seu em modo append-only, e a instância base recolhe uma cópia de cada secção para o seu próprio armazenamento. Serve para um nó a quem a instância base não pode oferecer espaço. É o único destino que precisa de uma porta aberta.

![O segundo passo com Guardá-los no nó escolhido, o endereço https://203.0.113.7:8443 e a nota sobre a porta aberta.](assets/screenshots/nodes/add-keep-dark.png#only-dark)
![O segundo passo com Guardá-los no nó escolhido, o endereço https://203.0.113.7:8443 e a nota sobre a porta aberta.](assets/screenshots/nodes/add-keep-light.png#only-light)

1. Em **Endereço do nó**, escreve onde a instância base chega ao nó: o seu endereço público e uma porta livre, como `https://203.0.113.7:8443`, sem pasta a seguir. Usa `http://` só com um endereço privado, como acima.
2. O nó arranca o container `bombvault-node-rest-server` nessa porta, com os seus repositórios na pasta `bombvault-node` dentro de Host Data. Com `https://` cria um certificado para esse endereço, e a instância base fixa-o.
3. Abre a porta na firewall do nó, para que a instância base lá consiga chegar. Uma porta que outro container do nó já ocupe é recusada, com o nome desse container.

A instância base recolhe uma secção logo a seguir ao fim dos backups do nó nessa secção, e como reserva a cada hora e um quarto, mesmo com a **Recolha** desligada. O que a instância base recolheu fica com ela como qualquer [cópia recolhida](offsite-recovery.md#receiver), mesmo depois de o nó desaparecer. O nó poda os seus próprios repositórios quando a instância base lho pede, já que o seu rest-server não aceita eliminações, e os seus conjuntos de pastas deixam de fora a pasta `bombvault-node`, para que nunca faça backup dos seus próprios backups.

## Quando os nós fazem backup {#schedule}

A instância base gere o agendamento de cada nó. O cartão mostra um plano por secção, como **Containers** *Diariamente às 2:00* e **Pastas** *Semanal (dom) às 4:00*. Um clique abre o mesmo editor que em [Agendamentos](schedules.md#plans), e uma alteração fica guardada de imediato.

![O plano dos containers de um nó aberto a partir do cartão, com Desativado, Diário, Semanal, A cada N dias e Cron.](assets/screenshots/nodes/schedule-dark.png#only-dark)
![O plano dos containers de um nó aberto a partir do cartão, com Desativado, Diário, Semanal, A cada N dias e Cron.](assets/screenshots/nodes/schedule-light.png#only-light)

- **Um nó que esteve ausente** inicia os backups que perdeu assim que volta. A instância base verifica a cada cinco minutos.
- **A um nó ocupado** volta a perguntar-se cinco minutos depois.
- **Enquanto a instância base poda** os backups antigos de um nó, o nó não inicia nada até ela acabar. No servidor recetor e no próprio nó, esta janela abre todos os domingos às 03:00 e dura até seis horas. Num destino, segue o plano de cópia externa da instância base para esse destino. A instância base guarda os backups pela sua própria regra para a secção em **Durante quanto tempo guardar**, ou pela regra do destino quando este tem uma.
- **Na lista de execuções do nó**, os backups e os restauros que a instância base iniciou aparecem como iniciados por ela.

## Acompanhar os teus nós {#watch}

O cartão **Nós** na visão geral lista cada nó com o seu estado, o último backup, os erros e as anomalias em aberto. Aparece assim que tens um nó.

![O cartão Nós na visão geral com três nós: um ligado, um inativo com dois erros e um ainda à espera do nó.](assets/screenshots/nodes/overview-card-dark.png#only-dark)
![O cartão Nós na visão geral com três nós: um ligado, um inativo com dois erros e um ainda à espera do nó.](assets/screenshots/nodes/overview-card-light.png#only-light)

A instância base envia um alerta por episódio pelos seus [canais de notificação](notifications.md), e o seguinte só depois de esse episódio terminar:

- **Um backup falhou**, por exemplo *BombVault: vps-helsinki: backup failed*, com a entrada e o motivo. Um backup posterior dessa entrada que corra bem termina o episódio.
- **O nó está fora de alcance**, por exemplo *BombVault: vps-helsinki is out of reach*. A instância base espera o tempo do interruptor de homem morto mais curto dos repositórios do nó, 26 horas se não o mudares, ou 6 horas enquanto o nó ainda não tem destino.

A monitorização do recetor da instância base trata também de cada repositório de um nó, com um interruptor de homem morto e um `restic check` diário às 04:00, vê [Recetor e recolhedor](offsite-recovery.md#receiver).

## As entradas de um nó {#entries}

Em **Backups**, as listas Containers e Pastas têm uma secção para cada nó por baixo das tuas próprias entradas. O container do próprio nó não aparece. **Fazer backup agora** no menu de uma entrada inicia o backup dela no nó, e **Abrir** abre a sua página.

Na secção de um nó da lista Pastas, **Adicionar conjunto de pastas** adiciona um conjunto de pastas no nó, e **Editar** no menu de um conjunto altera-o. Ambos abrem a mesma janela que para os teus próprios conjuntos. O seletor de pastas dessa janela percorre as pastas do nó, mas não cria pastas novas lá.

![A secção do nó vps-falkenstein na lista Containers: nextcloud, nextcloud-db, caddy e uptime-kuma.](assets/screenshots/nodes/backups-section-dark.png#only-dark)
![A secção do nó vps-falkenstein na lista Containers: nextcloud, nextcloud-db, caddy e uptime-kuma.](assets/screenshots/nodes/backups-section-light.png#only-light)

A página de uma entrada de um nó diz **Em** e o nome do nó, e lista os seus backups. **Restaurar** num deles restaura a entrada no nó e substitui o que lá está agora. Para isso o container é parado, como na instância base.

![A página do nextcloud em vps-falkenstein com os seus backups e um botão Restaurar ao lado de cada um.](assets/screenshots/nodes/entry-dark.png#only-dark)
![A página do nextcloud em vps-falkenstein com os seus backups e um botão Restaurar ao lado de cada um.](assets/screenshots/nodes/entry-light.png#only-light)

Através da instância base chegas a uma lista fixa de rotas do próprio nó: os seus containers, conjuntos de pastas, execuções e anomalias, e o backup e restauro deles. As definições, as chaves e o armazenamento do nó ficam fora de alcance, e nada nessa lista apaga. Vê [Os nós na API](api.md#nodes).

## Substituir e libertar {#replace-release}

**Substituir** mostra outra vez o ficheiro compose do nó, depois de um início de sessão com a palavra-passe. Usa-o para mudar o nó para outro servidor ou para o reconstruir: arranca o ficheiro lá e desliga o servidor antigo. A mesma `APP_KEY` e o mesmo `NODE_HOME` fazem dele outra vez o mesmo nó, com os seus backups e o seu histórico. Dois servidores com a mesma `APP_KEY` contam como um só nó, por isso nunca corras os dois.

**Libertar** deixa um nó ir embora. Deixa de receber ordens da instância base e não faz mais nenhum backup. Os seus backups ficam onde estão, tal como cada cópia que a instância base recolheu. O seu início de sessão no servidor recetor desaparece, e a instância base deixa de vigiar os seus repositórios. Um nó que esteja ausente quando o libertas mantém o cartão como **A ser libertado** até ficar a saber. Um nó libertado não aceita outra vez nenhuma instância base com a mesma `APP_KEY`. Para voltar a usar o servidor, adiciona-o como um nó novo e arranca-o com o ficheiro compose novo e uma pasta `config` vazia.

## Segurança {#security}

Se alguém tomar a instância base, controla também os seus nós, e um nó tem nas mãos o Docker do seu servidor, que lá equivale a root. Protege a instância base com uma palavra-passe e mantém-na fora da internet. Se alguém tomar um nó, não fica com nenhum grupo de emparelhamento, nenhuma fortaleza, nenhum outro nó nem nenhum outro repositório, e não consegue apagar o histórico no servidor recetor. Os pormenores estão em [Os nós no modelo de segurança](configuration.md#nodes).

O [kit de recuperação](offsite-recovery.md#recovery-kit) tem um capítulo **Nodes** com a `APP_KEY` de cada nó, a sua palavra-passe do restic e os seus repositórios, para poderes trazer um nó de volta ou restaurar os seus backups só com o restic.
