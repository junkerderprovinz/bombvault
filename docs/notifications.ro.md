# Notificări

BombVault te poate anunța când un backup eșuează, când reușește, când unul este întârziat și, o dată pe săptămână, cum a mers totul. Totul se setează la **Setări, Notificări**: ce declanșează un mesaj în cardul **Notificări**, unde ajung mesajele în cardul **Canale** și pingurile de check-in în cardul **Healthchecks**.

![Setări, Notificări cu canalul Gotify completat.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Setări, Notificări cu canalul Gotify completat.](assets/screenshots/notifications/gotify-light.png#only-light)

## Ce trimite un mesaj {#what}

Cardul **Notificări** stabilește regulile pentru toate canalele deodată:

- **Notifică**: **Niciodată**, **Doar la eșec** sau **La succes și eșec**, pentru fiecare canal de mai jos. O instalare nouă pornește pe **Doar la eșec**, așa că un canal pe care îl completezi îți spune când un backup se strică.
- **Rezumă rulările programate**: un mesaj pe rulare programată, de exemplu *42 din 45 reușite*, în loc de unul pentru fiecare container sau VM. Un backup pornit manual trimite în continuare propriul mesaj. Un oaspete înghețat, o retenție eșuată sau un buget de creștere depășit pleacă imediat, chiar și cu rezumatele pornite.
- **Notifică la actualizarea containerului**: un mesaj pentru fiecare container pe care **Actualizează containerele după o copie de rezervă reușită** l-a mutat pe o imagine mai nouă, ca să poți verifica dacă încă funcționează.
- **Raportează backupurile întârziate**: o dată pe zi la 09:00 BombVault verifică dacă un backup pornit este întârziat, adică mai vechi decât dublul programării sale, și trimite un mesaj pentru fiecare caz, care numește elementul rămas în urmă. Următorul backup reușit îl șterge.
- **Notificări Unraid**: trimite către sistemul propriu de notificări al Unraid, care poate redirecționa spre Pushover, e-mail sau Discord. Merge prin conexiunea SSH de la **Setări, Integrări, SSH al gazdei**, așa că acea cheie trebuie autorizată. Pe TrueNAS și pe alte gazde rămâne oprit.
- **Rezumat săptămânal**: un rezumat pe săptămână cu numărul de rulări, datele noi de backup, secțiunile întârziate, dacă copiile off-site sunt la zi și principalele eșecuri. Planul lui este un buton ca orice alt plan, iar **Oprit** îl dezactivează. Vezi [Programări](schedules.md#plans).

Ce anomalii trimit un mesaj se setează la **Setări, Verificare**, vezi [Anomalii](anomalies.md). Un backup amânat trimite un mesaj când începe să aștepte și unul când așteptarea se termină, nu unul la fiecare încercare.

**Trimite test**, în josul cardului, trimite un mesaj de test prin fiecare canal pornit, cu valorile așa cum sunt în formular, și numește fiecare canal care a eșuat. O verificare Healthchecks primește o intrare în jurnalul ei, care îi lasă starea neschimbată.

## Canale {#channels}

Fiecare canal are propriul comutator în cardul **Canale**, iar câmpurile lui apar odată ce este pornit. Fiecare canal trimite independent, cu propria limită de timp, așa că o gazdă care nu răspunde niciodată nu le blochează pe celelalte. O alertă de întârziere, de buget sau de anomalie pe care niciun canal nu a primit-o este încercată din nou, la fel și un rezumat săptămânal. Mesajul unui singur backup sau al unui rezumat este trimis o singură dată.

| Canal | Câmpuri | Note |
|---|---|---|
| **Webhook** | **URL Webhook**, **Format Webhook** | Formate: **JSON generic**, **Discord**, **Slack**, **Gotify** și **ntfy**. JSON generic trimite `title`, `message` și `ok`. |
| **Apprise** | **URL-ul API-ului Apprise**, **Etichete (opțional)** | Îndreaptă-l spre endpointul `/notify/<key>` al containerului tău Apprise API ca să ajungi la cele peste 100 de servicii ale lui. Etichetele aleg un subset din țintele acelei chei. |
| **Matrix** | **URL Homeserver**, **Token de acces**, **ID cameră** | Tokenul rămâne pe server și nu mai este afișat niciodată. |
| **Gotify** | **URL-ul serverului**, **Tokenul aplicației**, **Prioritate la eșec**, **Prioritate la succes** | Prioritățile merg de la 0 la 10 și încep de la 8 pentru eșecuri și 5 pentru succese. |
| **E-mail (SMTP)** | **Gazdă SMTP**, **Port**, **Criptare**, **Nume utilizator**, **Parolă**, **Adresă expeditor**, **Adresă destinatar** | **Criptare** este **STARTTLS**, **TLS (implicit)** sau **Fără**. |

Secretele, cum sunt tokenul Matrix, tokenul aplicației Gotify și parola SMTP, sunt stocate criptat. Sub un secret salvat câmpul spune **Salvat. Lasă câmpul gol ca să-l păstrezi.** Când schimbi adresa serverului, BombVault nu trimite vechiul secret către noul loc: câmpul îl cere din nou, iar noua adresă se salvează după ce îl introduci.

## Healthchecks {#healthchecks}

Un serviciu ca [Healthchecks.io](https://healthchecks.io) dă alarma când BombVault nu mai dă semn de viață. Lipește **URL de ping** al verificării, iar BombVault îl pinge pe tot parcursul fiecărui backup: la pornire, apoi la succes sau cu `/fail`. Face asta ori de câte ori este setat un URL, indiferent de ce spune **Notifică**, așa că verificarea rămâne verde la succes chiar și cu mesaje doar la eșec. O secțiune care nu are unde să facă backup contează ca eșuată, așa că verificarea ei devine roșie în loc să primească un succes.

Secțiunile cu același URL împart o verificare. După ce un backup al uneia a eșuat, backupurile reușite ale celorlalte pingează și ele `/fail` și o numesc, așa că verificarea rămâne roșie până când acel element are din nou un backup reușit sau până confirmi eșecul la **Necesită atenție**.

O secțiune poate face ping la o adresă proprie: **Excepție** din card adaugă una, iar rândul spune **Toate celelalte secțiuni fac ping la acest URL.**

## Exemplu: Gotify pe telefon {#example-gotify}

Rulezi Gotify la `https://gotify.example.com` și vrei ca eșecurile să sune pe telefon, iar succesele să sosească în liniște.

1. În Gotify, deschide **Apps**, creează o aplicație numită `BombVault` și copiază tokenul ei.
2. În BombVault, deschide **Setări, Notificări**. În **Notificări**, setează **Notifică** pe **La succes și eșec** și pornește **Rezumă rulările programate**, ca o noapte cu 45 de backupuri să trimită un singur mesaj în loc de 45.
3. În **Canale**, pornește **Gotify**.
4. Introdu `https://gotify.example.com` ca **URL-ul serverului**. BombVault adaugă singur `/message`.
5. Lipește tokenul în **Tokenul aplicației**.
6. Lasă **Prioritate la eșec** la 8 și **Prioritate la succes** la 5. Aplicația Android Gotify tratează 8 și mai mult ca ridicat, deci doar eșecurile fac zgomot.
7. Apasă **Trimite test**. Rândul de sub card spune dacă Gotify l-a primit, iar un token refuzat este numit ca atare.

Următoarea rulare programată trimite un rezumat cu prioritatea 5, sau 8 dacă ceva din ea a eșuat.

## Exemplu: ntfy ca webhook {#example-ntfy}

ntfy nu cere cont pentru un topic public. Alege un nume de topic greu de ghicit, pentru că oricine îl știe îl poate citi.

1. În **Canale**, pornește **Webhook**.
2. Introdu adresa topicului ca **URL Webhook**, de exemplu `https://ntfy.sh/bombvault-tower-7f3k`, sau adresa propriului tău server ntfy.
3. Setează **Format Webhook** pe **ntfy**.
4. Abonează-te la același topic în aplicația ntfy și apasă **Trimite test**.

BombVault trimite mesajul ca corp al cererii, cu titlul în `X-Title`. Un eșec pleacă cu prioritate ridicată și eticheta `warning`, ca să iasă în evidență în aplicație.

## Când mesajele nu ajung {#troubleshooting}

Vezi [Notificările nu ajung](troubleshooting.md#notifications-do-not-arrive) la Depanare.
