# Externo e recuperação

Os backups locais protegem-no de um container perdido ou de uma atualização má. A replicação externa e um kit de recuperação testado protegem-no da máquina inteira, de ransomware, ou de um incêndio. Esta página cobre replicar para o externo, tornar essa cópia à prova de adulteração, provar que consegue restaurar, e recuperar quando o próprio BombVault desaparece.

## Replicação externa

Mantenha o backup local rápido e adicione uma ou mais réplicas externas. Defina um repo por domínio no separador **Definições, Externo**. O BombVault replica novos instantâneos para lá com `restic copy` numa base de melhor esforço, por isso um percalço externo nunca faz o backup local falhar. O repo local mantém-se primário.

- **Vários destinos externos por domínio.** Cada domínio (containers, VMs, flash, config e conjuntos de ficheiros) pode replicar para vários destinos externos de uma só vez, não apenas um, para que possa manter, por exemplo, um rest-server na máquina de um amigo e um bucket S3 em paralelo. Adicione destinos extra em Definições, Externo, cada um com o seu próprio repositório, classe de armazenamento S3, flag append-only, retenção e orçamento de crescimento. Uma configuração externa única existente é transferida intacta como o primeiro destino, e cada destino de um domínio replica no agendamento externo desse domínio.
- **Agendamento externo por domínio** (editado ao lado de todos os outros agendamentos em Definições, Agendamentos): deixe-o em branco para replicar após cada backup local, ou defina uma cadência (por exemplo `weekly Sun 03:00`) para enviar para o externo com menos frequência do que faz backup localmente. Um botão **Replicar agora** cobre as execuções a pedido.
- **A retenção externa** vive em Definições, Externo para que possa manter as cópias externas por mais tempo como arquivo. Deixe a política toda a zero para nunca aparar automaticamente os instantâneos externos.
- **Os limites de largura de banda** (Definições, Externo) limitam a taxa de envio/receção do restic para que a replicação não sature a sua WAN.
- Um **indicador de replicação** mostra qual o domínio que está a replicar enquanto corre (na sua página e no Painel). É um indicador ativo, não uma barra de percentagem, porque o `restic copy` não expõe nenhum progresso legível por máquina.

!!! note "Restaurar de qualquer local"
    Cada container, VM, conjunto de ficheiros, a flash e a configuração da aplicação listam os seus backups como uma única linha do tempo por todos os locais onde um backup se encontra. Um backup copiado para o B2 aparece uma vez, marcado com cada local que o guarda. Um restauro usa o primeiro local a que consegue chegar, começando pelo repositório onde o item é escrito, e pode escolher outro local por linha. Os locais externos só são lidos quando os abre. Eliminar num local verifica primeiro os outros e diz se era a última cópia.

## Localização por item {#placement}

Cada cartão de container, VM e conjunto de ficheiros tem uma linha **Localização** com três segmentos:

- **Local** escreve o item no repositório mostrado em **Guardado em** e não o copia para lado nenhum. Use-o para dados que já têm uma segunda cópia, por exemplo uma partilha que vive num NAS.
- **Local + externo** escreve-o lá também e copia-o para os destinos marcados em **Copiar para**, um chip por destino externo do domínio. Desmarque um chip e esse destino deixa de receber algo de novo deste item.
- **Apenas externo** escreve o item diretamente no local em **Enviar para**: um repositório direto ao lado de um destino externo, ou um repositório remoto configurado em Definições, Caminhos e armazenamento, Repositórios.

A localização fica fixa desde o primeiro backup do item, porque o BombVault nunca move backups entre repositórios. As cópias podem mudar a qualquer momento. Um destino que deixa de receber um item mantém as cópias que tem e apara-as pela sua própria retenção na próxima execução externa do domínio; **Apagar em B2** no cartão remove-as de imediato. Quando algumas dessas cópias não existem em mais lado nenhum, a confirmação lista-as por data e pede o nome do item. De destinos append-only não se pode apagar.

Sob a linha, o cartão diz para onde vai o item e o que está lá de facto: quantos locais o guardam, quando cada destino foi visto pela última vez, e se o 3-2-1 é cumprido. Um local é o servidor com os dados originais, cada destino externo e cada repositório marcado **Fora das instalações**. O BombVault verifica cópias e locais; não verifica a parte dos «dois suportes» do 3-2-1.

### Localizações padrão

Definições, Caminhos e armazenamento, **Localizações padrão** tem uma linha por domínio com os mesmos três segmentos. As cópias aplicam-se de imediato a cada item sem escolha própria, e às pastas de projeto das stacks Compose. A localização aplica-se a um item novo no seu primeiro backup; alterá-la não move nenhum backup. Antes de guardar, a linha nomeia cada destino que ganha ou perde itens e quantos instantâneos isso significa. **Aplicar a itens sem backups** repõe no padrão todo o item que ainda não tem backup.

Um destino externo novo recebe todo o item que não está definido como Local. O diálogo que o adiciona diz quantos itens e, quando conhecido, quanto histórico isso representa, e propõe deixar de fora os itens já excluídos de outros destinos.

### Repositórios diretos

Escolher o repositório direto de um destino em Apenas externo abre um diálogo com uma localização sugerida junto ao destino, por exemplo `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, e um teste de ligação que não cria nada. **Criar e usar** cria o repositório e aponta o item para ele. Um repositório direto assume a chave, a classe de armazenamento, os limites, a definição append-only e a retenção do destino, e muda com eles; o cartão Repositórios mostra-o só de leitura. Quando uma chave nova do destino não consegue abri-lo, o repositório direto mantém a chave que tem e a gravação diz-o. Os seus instantâneos levam a etiqueta `bv:direct`, e todas as outras passagens de retenção mantêm-nos, por isso um repositório direto que perdeu a ligação ao seu destino nunca envelhece pelas regras locais. Acede-se ao B2 através do seu endpoint S3, indicando o ID da chave e a chave de aplicação como credenciais S3; uma chave limitada à pasta do próprio destino não consegue alcançar a pasta ao lado dela, por isso limite antes a chave à pasta acima do destino.

### Fora das instalações

Um repositório nomeado pode ser marcado **Fora das instalações** no cartão Repositórios. Os repositórios remotos começam marcados; desligue isto para um rest-server no mesmo edifício. A marca só conta para locais e para o 3-2-1 nos cartões. Não muda nenhuma cópia.

### Depois de uma reconstrução

As escolhas de cópia vivem nas próprias definições do BombVault. Depois de uma reconstrução através do Descobrir sem um `/config` restaurado, desaparecem, e copiar tudo voltaria a enviar para o B2 os itens que tinha deixado de fora. A replicação externa de cada domínio reconstruído entra por isso em pausa. O Painel mostra-o a âmbar, e as Localizações padrão oferecem **Confirmar padrão** com uma pré-visualização do que a próxima execução copia e os nomes nos backups que não têm entrada, que pode deixar de fora ali. Só a confirmação termina a pausa; importar um ficheiro de definições traz de volta regras e padrões mas não a termina.

## Repositórios primários remotos {#remote-primary-repositories}

O caminho de cópia de um domínio (Definições, Caminhos e armazenamento) não se limita a uma pasta local: aponta-o diretamente para um remoto restic (`s3:...`, `rest:http://host:8000/repo`, `sftp:utilizador@host:/repo`, `rclone:remoto:bucket/caminho`) e o BombVault copia diretamente para lá, sem cópia local separada e sem passo de replicação. É uma forma verdadeiramente diferente da replicação fora do local acima: ali o repositório local é o primário e o de fora do local é um arquivo dele na medida do possível; aqui o repositório remoto **é** o primário, e é a única cópia enquanto não configurares também uma replicação fora do local (ou um segundo remoto) para esse domínio.

Cada um dos cinco campos de caminho (Contentores, Máquinas virtuais, Flash, Configuração, Ficheiros) tem mesmo ao lado um interruptor **Local / Remoto**:

- **Local** mostra o explorador de pastas do costume.
- **Remoto** troca-o por um simples campo de URL, mais um botão que abre a mesma janela de teste de ligação e credenciais que os destinos fora do local usam, configurada para este primário. A partir daí obténs:
    - **Um teste de ligação** contra o caminho real, antes de dependeres dele.
    - **Limites de largura de banda** (envio e receção), para que uma cópia agendada para um primário remoto não sature a tua ligação WAN: os mesmos parâmetros restic `--limit-upload` e `--limit-download` que a replicação fora do local usa, aplicados à própria cópia.
    - **Proteção append-only (imutabilidade)**, verificada com o mesmo teste ativo de adulteração (uma sonda DELETE real contra o outro lado) que os destinos fora do local recebem. Com ela ligada, o BombVault recusa-se a podar o repositório: como atrás dele não há cópia local separada, as credenciais nesta máquina não podem ser capazes de apagar a única cópia da salvaguarda.
    - **Um alarme de orçamento de crescimento**, tirado da mesma tendência de tamanho do repositório que o cartão Armazenamento já acompanha.

Nada disto é obrigatório: um caminho remoto escrito à mão e sem definições de segurança guardadas copia exatamente como sempre (largura de banda ilimitada, podável, sem alarme de orçamento). A janela de segurança existe para quando quiseres as mesmas proteções que uma cópia fora do local recebe, sem teres de criar um destino fora do local só para isso.

!!! note "As credenciais de nuvem e REST são partilhadas"
    Um primário remoto autentica-se com as mesmas credenciais S3/REST configuradas em Definições, Fora do local, Credenciais de nuvem. Não há um cofre de credenciais separado para repositórios primários.

## Externo imutável (append-only)

Marque um repo externo como append-only para que ransomware, ou um host comprometido, não possa eliminar ou reescrever os seus backups. O lado remoto (um `restic/rest-server` a correr em modo `--append-only`) **impõe-no**. O BombVault apenas o **verifica** e nunca mostra verde só com base numa afirmação de configuração.

O assistente de **configuração guiada do externo** acompanha-o desde a escolha do backend (rest-server / rclone / S3), passando por um snippet de implementação de rest-server pronto a colar, um teste de ligação, o interruptor de imutabilidade (que corre o teste de adulteração de imediato) e uma estratégia de retenção, para que o externo append-only seja alcançável sem editar configs à mão.

!!! note "Uma exclusão bem-sucedida em `/locks/` é esperada"
    Append-only não significa que nada mais possa ser removido. O restic precisa criar e liberar os próprios bloqueios, por isso `/locks/` continua gravável e removível de propósito. Os snapshots e os dados por trás deles, exatamente o alvo de um ransomware, não podem ser removidos. Se você mesmo testar o lado remoto, uma exclusão bem-sucedida em `/locks/` é o comportamento correto e não uma falha.

!!! warning "Os repos imutáveis nunca são podados a partir desta máquina"
    Um externo imutável deliberadamente nunca poda instantâneos antigos. Defina um **alarme de orçamento de crescimento** para ele para ser alertado antes de o tamanho do repo descontrolar.

## Teste de adulteração

O BombVault prova periodicamente a garantia append-only tentando de facto uma eliminação contra o repo externo, dirigida a um objeto inexistente:

- **Recusada** significa protegido.
- **Aceite** significa não protegido.
- Um resultado **inconclusivo** (servidor inacessível, erro de autenticação) nunca inverte o veredicto guardado.

Uma inversão real de protegido-para-desprotegido dispara um único alerta.

## Ensaios de DR

O BombVault oferece dois níveis de prova de que os seus backups são de facto restauráveis, não apenas presentes.

- **Ensaios de verificação de restauro (local).** O BombVault corre periodicamente `restic check --read-data-subset` (limitado, nunca um restauro completo que enche o disco) e mostra um selo *último verificado como restaurável* por domínio. A cadência vive em Definições, Agendamentos; o selo em Definições, Integridade.
- **Ensaios de DR (externo).** O BombVault restaura um alvo real do repo externo para uma sandbox descartável, verifica-o ficheiro a ficheiro e byte a byte, e depois limpa. Isto prova que consegue recuperar do externo, não apenas que o repo responde.

O **scorecard de proteção contra ransomware** no Painel resume isto numa postura verde / âmbar / vermelha por domínio, com uma checklist com marca de idade (externo configurado, append-only verificado, replicação atual, ensaio de restauro passado, encriptação ligada, estratégia de poda definida). Cada linha vermelha liga diretamente à correção, e o cartão só fica verde com factos verificados.

## Painel recetor (o lado que recebe)

![O lado recetor, vigiado apenas para leitura, com uma verificação de integridade feita nesta máquina.](assets/screenshots/receiver.png)

*O lado recetor, vigiado apenas para leitura, com uma verificação de integridade feita nesta máquina.*

Tudo acima é o lado *emissor*. Na máquina que **recebe** cópias externas imutáveis de outro BombVault, o painel Recetor dá-lhe monitorização independente e só de leitura desses repositórios no hardware recetor, para que uma falha silenciosa no lado remoto não passe despercebida.

Ligue o interruptor **Recetor** em Definições para revelar um separador **Recetor**. Está desligado por predefinição; ative-o apenas numa máquina que de facto recebe backups externos imutáveis. Depois registe um repositório recebido (só de leitura, aberto com a chave da instância emissora) para obter:

- **Um inventário de instantâneos agrupado por origem**, para que possa ver exatamente quais containers, VMs e conjuntos de ficheiros aterraram.
- **Último recebido** por origem, para que saiba quão fresco cada um é.
- **Um `restic check` independente** corrido no hardware recetor, para que a integridade seja verificada onde os dados de facto residem, não apenas no emissor.
- **Um interruptor de homem-morto:** um alerta quando uma origem deixa de enviar dentro de uma janela que definir.
- **Alertas de integridade:** um alerta quando uma verificação no lado recetor falha.

O Recetor é estritamente só de leitura. Nunca escreve no repositório recebido, por isso nunca pode quebrar a garantia append-only da qual o emissor depende.

## Exemplo completo: duas máquinas Unraid, de ponta a ponta

Acima estão as peças. Isto é uma instalação completa com valores reais, porque as peças montam-se melhor depois de as termos visto montadas uma vez.

Duas máquinas: **TOWER** executa os contentores e envia as cópias, **VAULT** recebe-as e impõe a imutabilidade. Substitua pelos seus próprios nomes, endereços e caminhos de partilha.

**1. No VAULT, monte o servidor append-only.** No BombVault em TOWER vá a *Definições → Externo → configuração guiada*, escolha **rest-server** e gere a receita. Copie o separador **Modelo Unraid (XML)**, guarde-o no VAULT como `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, depois *Docker → Add Container* e escolha **rest-server** na lista de modelos. Antes de o iniciar, escreva a linha `htpasswd` mostrada em `/mnt/user/appdata/rest-server/.htpasswd` no VAULT. A palavra-passe de uso único é mostrada uma vez e nunca guardada: copie-a agora. Essa linha leva a mesma palavra-passe, já cifrada com bcrypt para ti: o texto em claro vai nas credenciais REST em TOWER, a linha cifrada no `.htpasswd` em VAULT. Não tens de cifrar nada.

    Deixe `--append-only` no campo OPTIONS. É esse o objetivo: sem ele, o VAULT volta a ser uma partilha comum.

**2. No TOWER, aponte o repositório externo para ele.** O URL do repositório segue o padrão que a receita imprime:

    rest:http://VAULT:8000/bombvault-containers/containers

O primeiro segmento do caminho é o utilizador htpasswd, o segundo é o repositório. Introduza o utilizador e a palavra-passe gerados como credenciais REST do destino e execute o **teste de ligação**.

**3. No TOWER, ative «Imutável».** O teste de adulteração corre de imediato e tem de dizer *protegido*. O que significam as respostas:

| Resultado | O que aconteceu |
| --- | --- |
| **protegido** | O VAULT recusou a eliminação. É o único estado que passa. |
| **NÃO protegido** | O VAULT aceitou uma eliminação. Falta `--append-only` ou foi retirado. |
| **inconclusivo** | Nem uma coisa nem outra. Normalmente o URL não é o que o restic usa, ou as credenciais mudaram. Nada é registado e nenhum alerta é disparado. |

**4. No VAULT, veja o que chega.** Ative *Definições → Recetor*, abra o separador **Recetor** e registe o repositório em apenas leitura.

!!! warning "A localização é um caminho **dentro** do contentor, escrito relativamente à montagem do anfitrião"
    Introduza `user/appdata/rest-server/bombvault-containers/containers`, e **não** `/mnt/user/appdata/…`. O BombVault corre num contentor onde o `/mnt` do anfitrião está montado noutro sítio; um caminho absoluto do anfitrião não existe lá. Se colar um, o BombVault indica-lhe agora o caminho relativo a usar.

    A **APP_KEY emissora** é a chave do TOWER, não a do VAULT. Encontra-a no TOWER em *Definições → Sistema*.

**5. Torne-o mútuo, se quiser.** Repita os mesmos cinco passos no sentido inverso: um rest-server no TOWER a receber a cópia do VAULT. Cada máquina impõe então a imutabilidade à outra, e nenhuma pode apagar as cópias da outra.

## Recuperação guiada

Um separador **Recuperação** dedicado acompanha uma instalação de raiz ou reconstruída pelo caso de desastre, num só lugar:

1. **Restaura primeiro as próprias definições do BombVault**, para que os caminhos de backup, os destinos externos e as credenciais de que o resto do fluxo precisa venham pré-preenchidos (aplicado através de um reinício automático sobre o socket Docker, para que a base de dados de definições em execução nunca seja sobrescrita sob um handle aberto).
2. **Verifica que o BombVault consegue ler os seus backups** (o senão da chave de encriptação logo à partida).
3. Deixa-o **apontar para o seu repo existente** (local ou externo).
4. **Descobre** os containers, VMs e conjuntos de ficheiros nele armazenados.
5. **Restaura-os todos** (deixados parados, para que os inicie deliberadamente), com o seu kit de recuperação a um clique de distância.

!!! note "As cópias externas esperam depois de uma reconstrução"
    Quando o passo 4 reconstrói entradas sem as definições antigas, a replicação externa desses domínios entra em pausa até a localização padrão ser confirmada. Consulte [Localização por item](#placement).

!!! tip "Migração planeada versus desastre"
    A recuperação guiada restaura as próprias definições do BombVault a partir de um backup. Para uma mudança *planeada* para uma máquina nova, pode em vez disso levar a sua configuração consigo diretamente com o cartão **Exportar e importar definições** (um ficheiro JSON portátil). Consulte [Configuração](configuration.md#portable-settings-export-and-import).

### Restaurar a partir de outro repo BombVault

Um cartão separado no separador **Recuperação** abre o repo de uma instância BombVault *diferente* (uma partilha montada sob `/mnt`, ou um URL remoto) com a **`APP_KEY` dessa instância**, numa sessão pontual e só de leitura. Navegue pelos containers, VMs e conjuntos de ficheiros lá armazenados, escolha um instantâneo e restaure-o, e o objeto restaurado torna-se um container, VM ou conjunto de ficheiros local normal. Nada é alguma vez escrito no outro repo, e as suas próprias definições de backup ficam intactas (a sessão vive em memória e expira por si própria). Mover um container do servidor A para o servidor B deixa de significar reapontar as suas definições de repo e revertê-las depois. A federação ao vivo servidor-a-servidor está explicitamente fora de âmbito; isto é um puxão pontual deliberado.

## Kit de recuperação da chave de encriptação

Esta é a peça que torna a recuperação de desastres possível mesmo quando não existe um BombVault em execução.

Um clique transfere a **chave mestra**, a **palavra-passe restic derivada**, e as **localizações e comandos exatos do repo**, para que possa restaurar diretamente com a CLI do restic em qualquer máquina. Um lembrete no Painel insiste até o ter guardado.

!!! danger "Guarde o kit de recuperação fora do servidor"
    O kit contém o segredo que decifra os seus backups. Guarde-o num local seguro e separado do servidor (um gestor de palavras-passe, uma cópia impressa num cofre). Se perder ambos o BombVault e a `APP_KEY` sem kit de recuperação, os seus backups encriptados não podem ser recuperados.

### Se não tiveres o kit à mão

A palavra-passe não está guardada em lado nenhum, é **calculada** a partir da `APP_KEY`. Com a chave e uma shell podes reproduzi-la tu próprio:

```sh
printf 'bombvault:restic-repo' \
  | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r \
  | cut -d' ' -f1
```

É um HMAC-SHA256 sobre a cadeia fixa `bombvault:restic-repo`, com os bytes crus da `APP_KEY` hexadecimal como chave, impresso como 64 caracteres hexadecimais minúsculos. O mesmo valor está no kit, como palavra-passe restic derivada; isto serve para o dia em que o kit esteja noutro sítio que não contigo.

!!! warning "Para um repositório recebido, usa a chave da instância REMETENTE"
    Um repositório que chegou aqui por replicação fora do local foi criado pela máquina que o enviou, com a **sua** `APP_KEY`. Derivar a partir da chave da máquina recetora dá uma palavra-passe que o restic recusa, o que se lê exatamente como um repositório corrompido sem o ser. É a razão habitual para o `restic check` num repositório recebido pedir a palavra-passe vezes sem conta.

Como as definições de recuperação vivem **dentro** de cada repo (`<repo>/def`, `<repo>/vm-def`), uma pasta de repo copiada é totalmente autossuficiente, por isso o kit mais o repo é tudo o que um restauro em bare-metal precisa.
