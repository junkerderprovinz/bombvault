# Aplicação Android

A aplicação Android leva todos os servidores BombVault do seu grupo para o telemóvel. Abre numa lista dos seus servidores com o registo de atividade de todos eles no topo, as mesmas linhas que o Painel mostra, e abre a vista de telemóvel do servidor em que tocar. A aplicação não faz nenhum backup por si.

## Obter a aplicação {#install}

- **Definições, Aplicações:** o cartão da aplicação Android oferece o APK da versão que o seu servidor executa, com um código QR para ler a partir do telemóvel.
- **APK:** cada versão tem `bombvault-android.apk` na sua página de release, e [esta ligação](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) descarrega sempre a compilação mais recente. Precisa de Android 10 ou posterior, e o Android pergunta uma vez se a aplicação com que abre o ficheiro pode instalar aplicações.
- **Google Play:** a aplicação está num teste fechado até poder ser pública. O Google Play só lista uma aplicação de uma conta de programador nova depois de pelo menos 12 testadores a terem mantido instalada durante 14 dias. Para ajudar, junte-se ao [grupo de testadores](https://groups.google.com/g/arrowloop-testers), abra a [página de teste](https://play.google.com/apps/testing/bombvault.halleluja.design), toque em **Tornar-se testador** e instale o BombVault a partir do Google Play.
- **F-Droid:** a listagem virá mais tarde.

Emparelhe a aplicação com servidores na versão 9.7.0 ou posterior. Um servidor numa versão mais antiga pode estar no mesmo grupo, mas mostra o telemóvel como uma instância normal, e a aplicação só lê a atividade desse servidor depois de iniciar sessão.

## Emparelhar por código QR {#pairing}

1. Em qualquer servidor do seu grupo, abra **Definições, Emparelhamento** e escolha **Mostrar frase**. As doze palavras aparecem com um código QR ao lado.
2. Na aplicação, toque em **Digitalizar código QR** e aponte o telemóvel para o código. Também pode colar ou escrever as palavras.
3. A aplicação lista os servidores desse grupo antes de guardar o que quer que seja. **Adicionar as N** adiciona-os todos.

A partir daí o telemóvel junta-se ao grupo como mais uma instância. Lê o que corre em cada servidor através do grupo sem iniciar sessão, diretamente em casa e através do retransmissor fora de casa. O funcionamento do próprio grupo está descrito em [Emparelhamento de instâncias](offsite-recovery.md#pairing).

!!! note "A interface continua a precisar de um caminho até ao servidor"
    A lista de servidores e o registo de atividade chegam através do grupo. A interface de um servidor abre diretamente, por isso o telemóvel tem de conseguir chegar ao endereço do servidor, em casa ou através de uma VPN.

## Sessão iniciada num telemóvel emparelhado {#sign-in}

Um telemóvel emparelhado com o seu grupo abre cada um dos seus servidores já com sessão iniciada. Antes de carregar uma página, pede uma sessão a esse servidor através do grupo, e um servidor só a concede a um membro que seja um telemóvel. Um servidor adicionado pelo endereço pede a palavra-passe, como num browser. Quem tiver as doze palavras já consegue abrir todos os backups do grupo, por isso o emparelhamento não concede nada de novo.

## Servidores fora de um grupo {#other-servers}

- **Adicionar servidor** aceita o endereço com que abre o BombVault num browser, por exemplo `192.168.1.10:3443`. Sem `http://` ou `https://` à frente, a aplicação usa https.
- Os servidores que se anunciam na rede local aparecem em **Nesta rede** e abrem com um toque. Fazem-no enquanto **Encontrar na rede** estiver ligado em Definições, Integrações. Um servidor noutra rede ou atrás de uma VPN não aparece aí.
- Um certificado autoassinado é aceite uma vez pela sua impressão digital SHA-256. Se mais tarde o servidor mostrar outro, a aplicação avisa-o e só o abre depois de confiar no novo certificado.

## O telemóvel na página Instâncias {#instances}

O telemóvel recebe um cartão próprio na página Instâncias de cada servidor do grupo, marcado como aplicação Android e com o nome que deu ao telemóvel. Não tem scorecard porque não faz backups, e **Remover** retira-o da página.

## Definições {#settings}

A roda dentada ao lado do botão + abre as definições da aplicação:

- o idioma e o nome que o telemóvel mostra na página Instâncias (vazio significa o modelo do telemóvel),
- o aspeto, que segue o primeiro servidor da lista até definir o seu, e as animações, que têm uma definição própria,
- um relatório para copiar quando comunicar um problema; não contém nenhum endereço, nome ou frase,
- o cartão **Sobre** com a política de privacidade,
- **Remover todos os servidores**, que retira todos os servidores da aplicação e sai do grupo. Nos próprios servidores nada muda.

## Transferências e envios {#files}

Exportações, kits de recuperação, ZIP do flash e dumps de bases de dados vão para a pasta Transferências do telemóvel, tal como a partir de um browser. Uma importação de definições abre o seletor de ficheiros do telemóvel.

## Num ecrã tátil {#touch}

Debaixo de um dedo não há hover, por isso um controlo escurece enquanto está premido, e um botão com o logótipo de uma marca acende-se na cor dessa marca até o dedo se levantar. Um toque longo num botão conta como um toque lento e não abre nenhum menu de ligação.

## Ou num browser {#browser}

O Chrome e o Edge conseguem instalar a interface web do BombVault como uma aplicação numa janela própria, tanto num telemóvel como num computador. Nada fica em cache, por isso uma atualização aparece de imediato.

## Privacidade {#privacy}

A aplicação não tem contas, publicidade nem análises, e não corre nada em segundo plano. A sua [política de privacidade](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) indica o que guarda e o que envia, e para onde.
