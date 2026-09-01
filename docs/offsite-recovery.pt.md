# Externo e recuperação

Os backups locais protegem-no de um container perdido ou de uma atualização má. A replicação externa e um kit de recuperação testado protegem-no da máquina inteira, de ransomware, ou de um incêndio. Esta página cobre replicar para o externo, tornar essa cópia à prova de adulteração, provar que consegue restaurar, e recuperar quando o próprio BombVault desaparece.

## Replicação externa

Mantenha o backup local rápido e adicione uma ou mais réplicas externas. Defina um repo por domínio no separador **Definições, Externo**. O BombVault replica novos instantâneos para lá com `restic copy` numa base de melhor esforço, por isso um percalço externo nunca faz o backup local falhar. O repo local mantém-se primário.

- **Vários destinos externos por domínio.** Cada domínio (containers, VMs, flash, config e conjuntos de ficheiros) pode replicar para vários destinos externos de uma só vez, não apenas um, para que possa manter, por exemplo, um rest-server na máquina de um amigo e um bucket S3 em paralelo. Adicione destinos extra em Definições, Externo, cada um com o seu próprio repositório, classe de armazenamento S3, flag append-only, retenção e orçamento de crescimento. Uma configuração externa única existente é transferida intacta como o primeiro destino, e cada destino de um domínio replica no agendamento externo desse domínio.
- **Agendamento externo por domínio** (editado ao lado de todos os outros agendamentos em Definições, Agendamentos): deixe-o em branco para replicar após cada backup local, ou defina uma cadência (por exemplo `weekly Sun 03:00`) para enviar para o externo com menos frequência do que faz backup localmente. Um botão **Replicar agora** cobre as execuções a pedido.
- **A retenção externa** vive em Definições, Externo para que possa manter as cópias externas por mais tempo como arquivo. Deixe a política toda a zero para nunca aparar automaticamente os instantâneos externos.
- **Os limites de largura de banda** (Definições, Externo) limitam a taxa de envio/receção do restic para que a replicação não sature a sua WAN.
- Um **indicador de replicação** mostra qual o domínio que está a replicar enquanto corre (na sua página e no Painel). É um indicador ativo, não uma barra de percentagem, porque o `restic copy` não expõe nenhum progresso legível por máquina.

!!! note "Restaurar diretamente do externo"
    Cada navegador de backups tem um interruptor **Local / Externo**, por isso, se um repo local se perder ou corromper, pode listar e restaurar diretamente a partir da réplica externa. A eliminação é por origem: remover um backup afeta apenas a cópia que está a ver.

## Repositórios primários remotos {#remote-primary-repositories}

O caminho de cópia de um domínio (Definições, Caminhos e armazenamento) não se limita a uma pasta local: aponta-o diretamente para um remoto restic (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:utilizador@host:/repo`, `rclone:remoto:bucket/caminho`) e o BombVault copia diretamente para lá, sem cópia local separada e sem passo de replicação. É uma forma verdadeiramente diferente da replicação fora do local acima: ali o repositório local é o primário e o de fora do local é um arquivo dele na medida do possível; aqui o repositório remoto **é** o primário, e é a única cópia enquanto não configurares também uma replicação fora do local (ou um segundo remoto) para esse domínio.

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

**1. No VAULT, monte o servidor append-only.** No BombVault em TOWER vá a *Definições → Externo → configuração guiada*, escolha **rest-server** e gere a receita. Copie o separador **Modelo Unraid (XML)**, guarde-o no VAULT como `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, depois *Docker → Add Container* e escolha **rest-server** na lista de modelos. Antes de o iniciar, escreva a linha `htpasswd` mostrada em `/mnt/user/appdata/rest-server/.htpasswd` no VAULT. A palavra-passe de uso único é mostrada uma vez e nunca guardada: copie-a agora.

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

!!! tip "Migração planeada versus desastre"
    A recuperação guiada restaura as próprias definições do BombVault a partir de um backup. Para uma mudança *planeada* para uma máquina nova, pode em vez disso levar a sua configuração consigo diretamente com o cartão **Exportar e importar definições** (um ficheiro JSON portátil). Consulte [Configuração](configuration.md#portable-settings-export-and-import).

### Restaurar a partir de outro repo BombVault

Um cartão separado no separador **Recuperação** abre o repo de uma instância BombVault *diferente* (uma partilha montada sob `/mnt`, ou um URL remoto) com a **`APP_KEY` dessa instância**, numa sessão pontual e só de leitura. Navegue pelos containers, VMs e conjuntos de ficheiros lá armazenados, escolha um instantâneo e restaure-o, e o objeto restaurado torna-se um container, VM ou conjunto de ficheiros local normal. Nada é alguma vez escrito no outro repo, e as suas próprias definições de backup ficam intactas (a sessão vive em memória e expira por si própria). Mover um container do servidor A para o servidor B deixa de significar reapontar as suas definições de repo e revertê-las depois. A federação ao vivo servidor-a-servidor está explicitamente fora de âmbito; isto é um puxão pontual deliberado.

## Kit de recuperação da chave de encriptação

Esta é a peça que torna a recuperação de desastres possível mesmo quando não existe um BombVault em execução.

Um clique transfere a **chave mestra**, a **palavra-passe restic derivada**, e as **localizações e comandos exatos do repo**, para que possa restaurar diretamente com a CLI do restic em qualquer máquina. Um lembrete no Painel insiste até o ter guardado.

!!! danger "Guarde o kit de recuperação fora do servidor"
    O kit contém o segredo que decifra os seus backups. Guarde-o num local seguro e separado do servidor (um gestor de palavras-passe, uma cópia impressa num cofre). Se perder ambos o BombVault e a `APP_KEY` sem kit de recuperação, os seus backups encriptados não podem ser recuperados.

Como as definições de recuperação vivem **dentro** de cada repo (`<repo>/def`, `<repo>/vm-def`), uma pasta de repo copiada é totalmente autossuficiente, por isso o kit mais o repo é tudo o que um restauro em bare-metal precisa.
