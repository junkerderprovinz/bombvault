# Aplicação Android

A aplicação Android leva todos os servidores BombVault do teu grupo para o telemóvel. Abre numa lista dos teus servidores com o registo de atividade de todos eles no topo, as mesmas linhas que a visão geral mostra, e abre a vista de telemóvel do servidor em que tocares. A aplicação não faz nenhum backup por si.

## Obter a aplicação {#install}

- **Definições, Aplicações:** o cartão da aplicação Android oferece o APK da versão que o teu servidor executa, com um código QR para ler a partir do telemóvel.
- **APK:** cada versão tem `bombvault-android.apk` na sua página de release, e [esta ligação](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) descarrega sempre a compilação mais recente. Precisa de Android 10 ou posterior, e o Android pergunta uma vez se a aplicação com que abre o ficheiro pode instalar aplicações.
- **Google Play:** a aplicação está num teste fechado até poder ser pública. O Google Play só lista uma aplicação de uma conta de programador nova depois de pelo menos 12 testadores a terem mantido instalada durante 14 dias. Para ajudar, junta-te ao [grupo de testadores](https://groups.google.com/g/arrowloop-testers), abre a [página de teste](https://play.google.com/apps/testing/bombvault.halleluja.design), toca em **Tornar-se testador** e instala o BombVault a partir do Google Play.
- **F-Droid:** a listagem virá mais tarde.

Emparelhe a aplicação com servidores na versão 9.7.0 ou posterior. Um servidor numa versão mais antiga pode estar no mesmo grupo, mas mostra o telemóvel como uma instância normal, e a aplicação só lê a atividade desse servidor depois de iniciar sessão.

## Emparelhar por código QR {#pairing}

1. Em qualquer servidor do teu grupo, abre **Definições, Emparelhamento** e escolhe **Mostrar frase**. As doze palavras aparecem com um código QR ao lado.
2. Na aplicação, toca em **Digitalizar código QR** e aponta o telemóvel para o código. Também podes colar ou escrever as palavras.
3. A aplicação lista os servidores desse grupo antes de guardar o que quer que seja. **Adicionar as N** adiciona-os todos.

A partir daí o telemóvel junta-se ao grupo como mais uma instância. Lê o que corre em cada servidor através do grupo sem iniciar sessão, diretamente em casa e através do retransmissor fora de casa. O funcionamento do próprio grupo está descrito em [Emparelhamento de instâncias](offsite-recovery.md#pairing).

!!! note "A interface continua a precisar de um caminho até ao servidor"
    A lista de servidores e o registo de atividade chegam através do grupo. A interface de um servidor abre diretamente, por isso o telemóvel tem de conseguir chegar ao endereço do servidor, em casa ou através de uma VPN.

## Sessão iniciada num telemóvel emparelhado {#sign-in}

Um telemóvel emparelhado com o teu grupo abre cada um dos servidores já com sessão iniciada. Antes de carregar uma página, pede uma sessão a esse servidor através do grupo, e um servidor só a concede a um membro que seja um telemóvel. Um servidor adicionado pelo endereço pede a palavra-passe, como num browser.
Um telemóvel emparelhado não recebe tudo o que uma palavra-passe dá. A sessão dele chega a uma lista fixa do que a aplicação mostra: a visão geral, as execuções e o registo de atividade, os backups para ver, iniciar e restaurar, as anomalias para ler e a página Instâncias para ver. Os destinos e os comandos voltam sem os endereços e os segredos. Um restauro que substitui o original faz primeiro backup do estado atual. As chaves, o kit de recuperação, os depósitos de chaves, as definições e a eliminação de backups ficam com um início de sessão no próprio servidor, e tudo o resto responde que precisa desse início de sessão.

O interruptor para isto é **Os telemóveis podem emparelhar com as doze palavras do grupo**, no cartão **App Android** em **Definições, Segurança**. Uma reposição da palavra-passe desativa-o e termina a sessão de todos os telemóveis, até o voltares a ativar aí.

## Servidores fora de um grupo {#other-servers}

- **Adicionar servidor** aceita o endereço com que abre o BombVault num browser, por exemplo `192.168.1.10:3443`. Sem `http://` ou `https://` à frente, a aplicação usa https.
- Os servidores que se anunciam na rede local aparecem em **Nesta rede** e abrem com um toque. Fazem-no enquanto **Encontrar na rede** estiver ligado no cartão **Serviços** em **Definições, Integrações**. Um servidor noutra rede ou atrás de uma VPN não aparece aí.
- Um certificado autoassinado é aceite uma vez pela sua impressão digital SHA-256. Se mais tarde o servidor mostrar outro, a aplicação avisa-te e só o abre depois de confiares no novo certificado.

## O telemóvel na página Instâncias {#instances}

O telemóvel recebe um cartão próprio na página Instâncias de cada servidor do grupo, marcado como aplicação Android e com o nome que deste ao telemóvel. Não tem scorecard porque não faz backups, e **Remover** retira-o da página.

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
