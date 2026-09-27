# Externo e recuperação

Os backups locais protegem-no de um container perdido ou de uma atualização má. A replicação externa e um kit de recuperação testado protegem-no da máquina inteira, de ransomware, ou de um incêndio. Esta página cobre replicar para o externo, tornar essa cópia à prova de adulteração, provar que consegue restaurar, e recuperar quando o próprio BombVault desaparece.

## Replicação externa

Mantenha o backup local rápido e copie-o para um ou mais outros lugares. Os lugares para onde um domínio é copiado escolhem-se no cartão **Domínios** em **Definições, Armazenamento**, um chip por lugar (consulte [Lugares de armazenamento](storage-places.md#domains)). O BombVault copia novos instantâneos para lá com `restic copy` numa base de melhor esforço, por isso uma cópia falhada nunca faz o backup local falhar. O lugar onde um domínio está guardado não tem de ser local; consulte [Um domínio guardado num lugar remoto](#remote-primary-repositories).

- **Vários lugares de cópia por domínio.** Um domínio pode ser copiado para vários lugares de uma só vez, por exemplo um rest-server em casa de um amigo e um bucket B2. A retenção, a classe de armazenamento, o append-only, os limites e o orçamento de crescimento pertencem ao lugar, por isso cada cópia segue as regras do lugar onde chega.
- **Agendamento de cópia por domínio** (editado ao lado de todos os outros agendamentos em Definições, Agendamentos): deixe-o em branco para copiar após cada backup local, ou defina uma cadência (por exemplo `weekly Sun 03:00`) para copiar com menos frequência do que faz backup. **Copiar agora** na linha do domínio corre-o a pedido.
- **Retenção por lugar.** Cada lugar tem as suas próprias regras, por isso um lugar externo pode manter as cópias por mais tempo como arquivo. Um lugar com todas as regras a zero nunca apara.
- **Os limites de largura de banda** por lugar limitam a taxa de envio e de receção do restic para que a cópia não sature a sua WAN.
- Um **indicador de replicação** mostra qual o domínio que está a copiar enquanto corre (na sua página e no Painel). É um indicador ativo, não uma barra de percentagem, porque o `restic copy` não expõe nenhum progresso legível por máquina.

!!! note "Restaurar de qualquer local"
    Cada container, VM, conjunto de ficheiros, a flash e a configuração da aplicação listam os seus backups como uma única linha do tempo por todos os locais onde um backup se encontra. Um backup copiado para o B2 aparece uma vez, marcado com cada local que o guarda. Um restauro usa o primeiro local a que consegue chegar, começando pelo repositório onde o item é escrito, e pode escolher outro local por linha. Os locais externos só são lidos quando os abre. Eliminar num local verifica primeiro os outros e diz se era a última cópia.

## Localização por item {#placement}

Cada cartão de container, VM e conjunto de ficheiros tem uma linha **Localização** com três segmentos:

- **Local** escreve o item no repositório mostrado em **Guardado em** e não o copia para lado nenhum. Use-o para dados que já têm uma segunda cópia, por exemplo uma partilha que vive num NAS.
- **Local + externo** escreve-o lá também e copia-o para os destinos marcados em **Copiar para**, um chip por destino externo do domínio. Desmarque um chip e esse destino deixa de receber algo de novo deste item.
- **Apenas externo** escreve o item diretamente no lugar em **Enviar para**, qualquer lugar que não seja o lugar principal do domínio. Quando o domínio já é copiado para esse lugar, o item recebe um repositório direto ao lado das cópias; caso contrário, o BombVault cria lá um repositório para o domínio.

A localização fica fixa desde o primeiro backup do item, porque o BombVault nunca move backups entre repositórios. As cópias podem mudar a qualquer momento. Um destino que deixa de receber um item mantém as cópias que tem e apara-as pela sua própria retenção na próxima execução externa do domínio; **Apagar em B2** no cartão remove-as de imediato. Quando algumas dessas cópias não existem em mais lado nenhum, a confirmação lista-as por data e pede o nome do item. De destinos append-only não se pode apagar.

Sob a linha, o cartão diz para onde vai o item e o que está lá de facto: quantos locais o guardam, quando cada destino foi visto pela última vez, e se o 3-2-1 é cumprido. Um local é o servidor com os dados originais e cada lugar noutro local (consulte [Fora das instalações](#off-the-premises-mark)). O BombVault verifica cópias e locais; não verifica a parte dos «dois suportes» do 3-2-1.

### Padrões por domínio

O cartão **Domínios** em Definições, Armazenamento tem uma linha por domínio. **Copiado para** aplica-se de imediato a cada item sem escolha própria, e às pastas de projeto das stacks Compose. Depois de um domínio ter backups, **Guardado em** aplica-se a um item novo no seu primeiro backup, e alterá-lo não move nenhum backup. Antes de guardar, a linha nomeia cada lugar que ganha ou perde itens e quantos instantâneos isso significa, e a pergunta traz o interruptor **Aplicar a itens sem backups**, que também põe no novo padrão todo o item que ainda não tem backup. **Exceções** lista os itens com escolha própria.

Marcar um lugar novo em **Copiado para** faz com que receba todo o item que não está definido como Local. A confirmação diz quantos itens e, quando conhecido, quanto histórico isso representa.

### Repositórios diretos

Escolher em Apenas externo um lugar para onde o domínio já é copiado pede confirmação uma vez e depois cria um repositório direto ao lado das cópias, por exemplo `s3:https://s3.eu-central-003.backblazeb2.com/bucket/container-direct`, e aponta o item para ele. Para um destino de cópia sem lugar, a escolha abre um diálogo com um endereço sugerido e um teste de ligação que não cria nada, e **Criar e usar** cria o repositório. Um repositório direto assume a chave, a classe de armazenamento, os limites, a definição append-only e a retenção do lugar, e muda com eles. Quando uma chave nova do lugar não consegue abri-lo, o repositório direto mantém a chave que tem e a gravação diz-o. Os seus instantâneos levam a etiqueta `bv:direct`, e todas as outras passagens de retenção mantêm-nos, por isso um repositório direto que perdeu a ligação ao seu lugar nunca envelhece pelas regras locais. Uma chave B2 limitada a uma pasta tem de cobrir o endereço do lugar, e não só a pasta do domínio, senão a pasta ao lado fica fora de alcance.

### Fora das instalações {#off-the-premises-mark}

Uma cópia só conta como um local à parte quando o seu lugar está noutro local. Um lugar na nuvem conta sempre e uma pasta neste Unraid nunca conta; para um NAS, um rest-server ou um servidor SFTP, responda a **Onde está o dispositivo?** nos detalhes do lugar com **Aqui em casa** ou **Noutro local**. A resposta só conta para os locais e para o 3-2-1 nos cartões e no Painel. Não muda nenhuma cópia.

### Depois de uma reconstrução

As escolhas de cópia vivem nas próprias definições do BombVault. Depois de uma reconstrução através do Descobrir sem um `/config` restaurado, desaparecem, e copiar tudo voltaria a enviar para o B2 os itens que tinha deixado de fora. A replicação externa de cada domínio reconstruído entra por isso em pausa. O Painel mostra-o a âmbar, e a linha do domínio no cartão Domínios oferece **Confirmar padrão** com uma pré-visualização do que a próxima execução copia e os nomes nos backups que não têm entrada, que pode deixar de fora ali. Só a confirmação termina a pausa; importar um ficheiro de definições traz de volta regras e padrões mas não a termina.

## Um domínio guardado num lugar remoto {#remote-primary-repositories}

Um domínio não tem de ser guardado localmente. Enquanto a sua localização de backup não tiver backups, escolha um lugar remoto em **Guardado em** no cartão Domínios e o domínio faz backup diretamente para lá, sem cópia local e sem passo de cópia. O repositório remoto é então a única cópia, a menos que o domínio também seja copiado para outro lugar. Todos os lugares remotos trazem as mesmas salvaguardas:

- **Um teste de ligação** antes de qualquer coisa ser escrita.
- **Limites de largura de banda** para o próprio backup, os mesmos parâmetros `--limit-upload` e `--limit-download` que uma cópia usa.
- **Proteção append-only**, verificada com o mesmo teste ativo de adulteração. Com ela ligada, o BombVault nunca poda o repositório, porque as credenciais nesta máquina não podem ser capazes de apagar a única cópia do backup.
- **Um orçamento de crescimento**, tirado da mesma tendência de tamanho que o cartão Armazenamento acompanha.

Um domínio guardado num lugar remoto é a origem das suas cópias, tal como um domínio guardado localmente; consulte [Cópias entre lugares com credenciais diferentes](storage-places.md#different-credentials).

!!! note "As credenciais pertencem ao lugar"
    Um lugar remoto tem as suas próprias credenciais. Um lugar configurado com as credenciais de nuvem partilhadas continua a usá-las até o seu acesso ser alterado nos seus detalhes.

## Externo imutável (append-only)

Marque um repo externo como append-only para que ransomware, ou um host comprometido, não possa eliminar ou reescrever os seus backups. O lado remoto (um `restic/rest-server` a correr em modo `--append-only`) **impõe-no**. O BombVault apenas o **verifica** e nunca mostra verde só com base numa afirmação de configuração.

A janela **Adicionar lugar** traz uma receita pronta a colar para um rest-server em modo append-only, com um utilizador para este BombVault. Num lugar rest-server com **Append-only** ligado, **Testar append-only** nos detalhes do lugar corre o teste de adulteração contra cada caminho de domínio, cada cópia ligada e cada repositório no lugar e dá uma única resposta para o lugar, para que o externo append-only seja alcançável sem editar configs à mão.

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

Num lugar, **Testar append-only** testa cada caminho de domínio, cada cópia ligada e cada repositório lá com as respetivas credenciais e junta os veredictos numa única resposta: basta um repositório aceitar uma eliminação para o lugar inteiro ficar *eliminações aceites*.

## Ensaios de DR

O BombVault oferece dois níveis de prova de que os seus backups são de facto restauráveis, não apenas presentes.

- **Ensaios de verificação de restauro (local).** O BombVault corre periodicamente `restic check --read-data-subset` (limitado, nunca um restauro completo que enche o disco) e mostra um selo *último verificado como restaurável* por domínio. A cadência vive em Definições, Agendamentos; o selo em Definições, Integridade.
- **Ensaios de DR (externo).** O BombVault restaura um alvo real do repo externo para uma sandbox descartável, verifica-o ficheiro a ficheiro e byte a byte, e depois limpa. Isto prova que consegue recuperar do externo, não apenas que o repo responde. Só são ensaiados lugares noutro local, porque uma cópia na mesma casa não prova nada sobre perder a casa. Um domínio copiado para vários deles é ensaiado contra um por cada execução agendada, à vez, e o Painel indica o lugar do último ensaio.

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

**1. No VAULT, monte o servidor append-only.** No BombVault em TOWER abra *Definições → Armazenamento*, clique em **Adicionar lugar**, escolha **rest-server** e clique em **Mostrar receita**. Copie o bloco **Modelo do Unraid**, guarde-o no VAULT como `/boot/config/plugins/dockerMan/templates-user/my-rest-server.xml`, depois *Docker → Add Container* e escolha **rest-server** na lista de modelos. Antes de o iniciar, escreva a linha `htpasswd` mostrada em `/mnt/user/appdata/rest-server/.htpasswd` no VAULT. A palavra-passe é mostrada uma vez e nunca guardada; a receita já a colocou, com o utilizador, no formulário em TOWER, por isso deixe essa janela aberta. A linha `htpasswd` leva a mesma palavra-passe, já cifrada com bcrypt para si, por isso não tem de cifrar nada.

    Deixe `--append-only` no campo OPTIONS. Sem ele, o VAULT volta a ser apenas uma partilha comum.

**2. No TOWER, adicione o lugar.** Introduza o endereço do VAULT, `http://VAULT:8000`, ao lado do utilizador e da palavra-passe que a receita preencheu, e depois clique em **Testar ligação**. O BombVault constrói o endereço a partir deles:

    rest:http://VAULT:8000/tower

O primeiro segmento do caminho é o utilizador htpasswd, aqui `tower`, e cada domínio recebe a sua pasta por baixo dele, por exemplo `rest:http://VAULT:8000/tower/container`. Responda a **Onde está o dispositivo?** com **Noutro local**, clique em **Adicionar** e marque o lugar em **Copiado para** nos domínios que devem ir para lá.

**3. No TOWER, ligue Append-only** em **Proteção** nos detalhes do lugar e depois clique em **Testar append-only**. O teste verifica cada caminho de domínio, cada cópia e cada repositório no lugar e dá uma única resposta para o lugar, que tem de ser *eliminações recusadas*. O que significam as respostas:

| Resultado | O que aconteceu |
| --- | --- |
| **eliminações recusadas** | O VAULT recusou a eliminação. É o único estado que passa. |
| **eliminações aceites** | O VAULT aceitou uma eliminação. Falta `--append-only` ou foi retirado. |
| uma mensagem em vez de um resultado | O teste não conseguiu correr. Normalmente o endereço não é o que o próprio restic usa, ou as credenciais mudaram. Nada é registado e nenhum alerta é disparado. |

**4. No VAULT, veja o que chega.** Ative *Definições → Recetor*, abra o separador **Recetor** e registe o repositório em apenas leitura.

!!! warning "A localização é um caminho **dentro** do contentor, escrito relativamente à montagem do anfitrião"
    Introduza `user/appdata/rest-server/tower/container`, e **não** `/mnt/user/appdata/…`. O BombVault corre num contentor onde o `/mnt` do anfitrião está montado noutro sítio; um caminho absoluto do anfitrião não existe lá. Se colar um, o BombVault indica-lhe o caminho relativo a usar.

    A **APP_KEY emissora** é a chave do TOWER, não a do VAULT. Encontra-a no TOWER em *Definições → Sistema*.

**5. Torne-o mútuo, se quiser.** Repita os mesmos cinco passos no sentido inverso: um rest-server no TOWER a receber a cópia do VAULT. Cada máquina impõe então a imutabilidade à outra, e nenhuma pode apagar as cópias da outra.

## Recuperação guiada

Um separador **Recuperação** dedicado acompanha uma instalação de raiz ou reconstruída pelo caso de desastre, num só lugar:

1. **Verifica que o BombVault consegue ler os seus backups** (o senão da chave de encriptação logo à partida).
2. **Restaura as próprias definições do BombVault**, para que os caminhos de backup, os destinos externos e as credenciais de que o resto do fluxo precisa venham pré-preenchidos. O backup de definições é lido do lugar que a linha Auto-backup indica em **Guardado em**, ou da cópia do Auto-backup em **Copiado para**, e o passo mostra esse lugar com o seu endereço; para ler de outro lugar, altere primeiro a linha Auto-backup no passo 3. O restauro é aplicado através de um reinício automático sobre o socket Docker, para que a base de dados de definições em execução nunca seja sobrescrita sob um handle aberto.
3. **Anexa os seus backups existentes** através das linhas do cartão Domínios: na linha de cada domínio, escolha em **Guardado em** o lugar onde estão os backups dele e em **Copiado para** os lugares que guardam as cópias. Um lugar que ainda nenhuma linha oferece, como uma partilha, um servidor ou um bucket na nuvem, liga-se com **Adicionar lugar**, a mesma janela que em Definições, Armazenamento. **Ligar e pré-visualizar** verifica depois se os backups podem ser lidos.
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
