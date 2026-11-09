# Notificări

BombVault te poate anunța când un backup eșuează, când reușește, când unul este întârziat și, o dată pe săptămână, cum a mers totul. Totul se setează la **Setări, Notificări**: ce trimite BombVault în afară de backupurile individuale, în cardul **Notificări**, și unde ajung mesajele, în cardul **Canale**. Fiecare canal decide singur când dă de veste, iar o verificare Healthchecks este și ea un canal.

![Setări, Notificări cu canalul Gotify completat.](assets/screenshots/notifications/gotify-dark.png#only-dark)
![Setări, Notificări cu canalul Gotify completat.](assets/screenshots/notifications/gotify-light.png#only-light)

## Ce trimite un mesaj {#what}

Comutatoarele din cardul **Notificări** se aplică fiecărui canal:

- **Rezumă rulările programate**: un mesaj pe rulare programată, de exemplu *42 din 45 reușite*, în loc de unul pentru fiecare container sau VM. Un backup pornit manual trimite în continuare propriul mesaj. Un oaspete înghețat, o retenție eșuată sau un buget de creștere depășit pleacă imediat, chiar și cu rezumatele pornite.
- **Notifică la actualizarea containerului**: un mesaj pentru fiecare container pe care **Actualizează containerele după o copie de rezervă reușită** l-a mutat pe o imagine mai nouă, ca să poți verifica dacă încă funcționează.
- **Raportează backupurile întârziate**: o dată pe zi la 09:00 BombVault verifică dacă un backup pornit este întârziat, adică mai vechi decât dublul programării sale, și trimite un mesaj pentru fiecare caz, care numește elementul rămas în urmă. Următorul backup reușit îl șterge.
- **Rezumat săptămânal**: un rezumat pe săptămână cu numărul de rulări, datele noi de backup, secțiunile întârziate, dacă copiile off-site sunt la zi și principalele eșecuri. Planul lui este un buton ca orice alt plan, iar **Oprit** îl dezactivează. Vezi [Programări](schedules.md#plans).

Dacă un canal află și de succese se alege chiar pe canal, la **Notifică**. Ce anomalii trimit un mesaj se setează la **Setări, Verificare**, vezi [Anomalii](anomalies.md). Un backup amânat trimite un mesaj când începe să aștepte și unul când așteptarea se termină, nu unul la fiecare încercare.

## Canale {#channels}

**Adaugă canal**, în josul cardului **Canale**, oferă **Notificări Unraid**, **ntfy**, **Gotify**, **Discord**, **Slack**, **Matrix**, **E-mail (SMTP)**, **Apprise**, **Webhook** și **Healthchecks**. Adaugă câte vrei, chiar și două de același tip, de exemplu un topic ntfy pentru eșecuri și altul pentru tot. Un canal nou pornește oprit, ca să fie salvat înainte să-i completezi câmpurile.

Fiecare canal are:

- propriul comutator, care îl oprește fără să-i piardă câmpurile.
- **Notifică**: **Niciodată**, **Doar la eșec** sau **La succes și eșec**. Un canal nou pornește pe **Doar la eșec**. Un backup care a reușit, dar a omis ceva, ajunge și la un canal setat doar pe eșecuri.
- **Nume**, opțional, care apare în locul tipului, util când ai două canale de același tip.
- **Trimite test**, care trimite un mesaj de test prin acest canal cu valorile așa cum sunt în formular, salvate sau nu. Rândul de sub canal spune dacă serviciul l-a primit și, dacă nu, de ce.
- **Elimină canalul**, coșul de gunoi de lângă el.

Fiecare canal trimite independent, cu propria limită de timp, așa că o gazdă care nu răspunde niciodată nu le blochează pe celelalte. Când un serviciu eșuează într-un fel pe care o nouă încercare l-ar putea rezolva, canalul încearcă de până la trei ori. O alertă de întârziere, de buget sau de anomalie pe care niciun canal nu a primit-o este încercată din nou mai târziu, la fel și un rezumat săptămânal.

| Canal | Câmpuri | Note |
|---|---|---|
| **Notificări Unraid** | niciunul | Trimite către sistemul propriu de notificări al Unraid, care poate redirecționa spre Pushover, e-mail sau Discord. Merge prin conexiunea SSH de la **Setări, Integrări, SSH al gazdei**, așa că acea cheie trebuie autorizată. Pe TrueNAS și pe alte gazde rămâne tăcut, iar canalul spune de ce. |
| **ntfy** | **URL-ul subiectului**, **Token de acces (opțional)** | Adresa se termină cu topicul tău. Un token e necesar doar pentru un topic protejat. |
| **Gotify** | **URL-ul serverului**, **Tokenul aplicației**, **Prioritate la eșec**, **Prioritate la succes** | Prioritățile merg de la 0 la 10 și încep de la 8 pentru eșecuri și 5 pentru succese. |
| **Discord** | **URL Webhook** | Adresa unui webhook din setările canalului Discord. |
| **Slack** | **URL Webhook** | Adresa unui webhook de intrare al aplicației tale Slack. |
| **Matrix** | **URL Homeserver**, **Token de acces**, **ID cameră** | Contul din spatele tokenului trebuie să fie în cameră. |
| **E-mail (SMTP)** | **Gazdă SMTP**, **Port**, **Criptare**, **Nume utilizator**, **Parolă**, **Adresă expeditor**, **Adresă destinatar** | **Criptare** este **STARTTLS**, **TLS (implicit)** sau **None**. |
| **Apprise** | **URL-ul API-ului Apprise**, **Etichete (opțional)**, **Adrese Apprise (opțional)** | Îndreaptă-l spre endpointul `/notify/<key>` al containerului tău Apprise API ca să ajungi la cele peste 100 de servicii ale lui, iar etichetele aleg un subset din țintele acelei chei. Sau trece adrese Apprise, câte una pe rând, de exemplu `tgram://` pentru Telegram sau `pover://` pentru Pushover, și termină URL-ul API în `/notify`, fără cheie. |
| **Webhook** | **URL Webhook**, **Metodă**, **Anteturi**, **Conținut** | O cerere pe care o construiești singur, vezi [Webhook](#webhook). |
| **Healthchecks** | **URL de ping** | Vezi [Healthchecks](#healthchecks). |

Secretele, cum sunt tokenurile, parola SMTP și valorile anteturilor de webhook, sunt stocate criptat, iar serverul nu le trimite niciodată înapoi paginii. Sub un secret salvat câmpul spune **Salvat. Lasă câmpul gol ca să-l păstrezi.** Când schimbi unde trimite un canal, BombVault nu trimite vechiul secret către noul loc: câmpul spune **Adresa nouă se salvează când introduci din nou asta.**, iar noua adresă se salvează după ce îl introduci.

Setările dintr-o versiune de dinainte de lista de canale sunt preluate la prima pornire și nu se pierde nimic. Fiecare canal pe care îl completaseși devine o intrare separată, cu alegerea de la **Notifică** pe care o aveai. Un webhook setat pe formatul Discord, Slack, Gotify sau ntfy devine un canal de acel tip, iar un webhook JSON generic trimite în continuare `title`, `message` și `ok` prin **Conținut**. Comutatorul Unraid devine un canal Unraid, iar URL-ul Healthchecks cu excepțiile lui devine un canal Healthchecks.

### Webhook {#webhook}

Un canal **Webhook** trimite o cerere HTTP către orice adresă. **Metodă** este GET, POST sau PUT. **Adaugă antet** adaugă un rând sub **Anteturi**, iar valorile anteturilor sunt păstrate ca secrete, așa că un antet `Authorization` e în siguranță acolo. **Conținut** este trimis exact cum îl scrii, cu acești substituenți completați:

| Substituent | Devine |
|---|---|
| `%%title%%` | titlul mesajului |
| `%%body%%` | mesajul propriu-zis |
| `%%ok%%` | `true`, sau `false` la un eșec |
| `%%event%%` | ce s-a întâmplat, de exemplu `container.failed` |
| `%%severity%%` | `info`, `success`, `warning`, `failure` sau `critical` |
| `%%importance%%` | nivelul Unraid pentru el: `normal`, `warning` sau `alert` |
| `%%link%%` | un link care ține de mesaj, gol când nu există |
| `%%key%%` | secțiunea, de exemplu `container` sau `VM` |
| `%%time%%` | ora în UTC, de exemplu `2026-11-09T03:00:00Z` |
| `%%source%%` | `BombVault` |

Substituenții funcționează și în adresă și în valorile anteturilor, iar fiecare valoare este codificată potrivit locului în care stă: ca JSON când antetul `Content-Type` indică JSON și ca parametru de interogare în adresă. Un webhook nou pornește cu acest antet și trimite `{"title":"%%title%%","message":"%%body%%","ok":%%ok%%}`. Un nume pe care BombVault nu îl cunoaște rămâne cum l-ai scris, așa că o greșeală de tastare se vede în mesaj. Lasă **Conținut** gol ca să nu trimiți niciun conținut.

## Healthchecks {#healthchecks}

Un serviciu ca [Healthchecks.io](https://healthchecks.io) dă alarma când BombVault nu mai dă semn de viață. Adaugă un canal **Healthchecks** și lipește **URL de ping** al verificării, iar BombVault îl pinge pe tot parcursul fiecărui backup: la pornire, apoi la succes sau cu `/fail`. Acest canal nu are **Notifică**, pentru că o verificare are nevoie de pingurile de succes ca să rămână verde. O secțiune care nu are unde să facă backup contează ca eșuată, așa că verificarea ei devine roșie în loc să primească un succes.

Secțiunile cu același URL împart o verificare. După ce un backup al uneia a eșuat, backupurile reușite ale celorlalte pingează și ele `/fail` și o numesc, așa că verificarea rămâne roșie până când acel element are din nou un backup reușit sau până confirmi eșecul la **Necesită atenție**.

O secțiune poate face ping la o adresă proprie: **Excepție** din canalul Healthchecks adaugă una, iar rândul spune atunci **Toate celelalte secțiuni fac ping la acest URL.** **Trimite test** scrie o intrare în jurnalul fiecărei verificări, care îi lasă starea neschimbată.

## Exemplu: Gotify pe telefon {#example-gotify}

Rulezi Gotify la `https://gotify.example.com` și vrei ca eșecurile să sune pe telefon, iar succesele să sosească în liniște.

1. În Gotify, deschide **Apps**, creează o aplicație numită `BombVault` și copiază tokenul ei.
2. În BombVault, deschide **Setări, Notificări** și pornește **Rezumă rulările programate**, ca o noapte cu 45 de backupuri să trimită un singur mesaj în loc de 45.
3. În **Canale**, apasă **Adaugă canal** și alege **Gotify**.
4. Introdu `https://gotify.example.com` ca **URL-ul serverului**. BombVault adaugă singur `/message`.
5. Lipește tokenul în **Tokenul aplicației**.
6. Setează **Notifică** pe **La succes și eșec**. Lasă **Prioritate la eșec** la 8 și **Prioritate la succes** la 5. Aplicația Android Gotify tratează 8 și mai mult ca ridicat, deci doar eșecurile fac zgomot.
7. Pornește canalul și apasă **Trimite test**. Rândul de sub canal spune dacă Gotify l-a primit, iar un token refuzat este numit ca atare.

Următoarea rulare programată trimite un rezumat cu prioritatea 5, sau 8 dacă ceva din ea a eșuat.

## Exemplu: ntfy fără cont {#example-ntfy}

ntfy nu cere cont pentru un topic public. Alege un nume de topic greu de ghicit, pentru că oricine îl știe îl poate citi.

1. În **Canale**, apasă **Adaugă canal** și alege **ntfy**.
2. Introdu adresa topicului ca **URL-ul subiectului**, de exemplu `https://ntfy.sh/bombvault-tower-7f3k`, sau un topic de pe propriul tău server ntfy. Lasă **Token de acces (opțional)** gol pentru un topic public.
3. Abonează-te la același topic în aplicația ntfy, pornește canalul și apasă **Trimite test**.

BombVault trimite mesajul ca corp al cererii, cu titlul în antetul `Title`. Un eșec pleacă cu prioritate ridicată și eticheta `warning`, ca să iasă în evidență în aplicație, iar o VM rămasă înghețată pleacă drept urgentă.

## Când mesajele nu ajung {#troubleshooting}

Vezi [Notificările nu ajung](troubleshooting.md#notifications-do-not-arrive) la Depanare.
