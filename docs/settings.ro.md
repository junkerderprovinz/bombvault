# Setări

**Setările** sunt paisprezece plăci. Fiecare placă este o pagină de carduri, iar fiecare schimbare se scrie pe loc, cu o scurtă licărire pe control sau cu un tremur când serverul o refuză. Câteva ferestre păstrează un buton **Salvează** pentru că salvarea lor pe jumătate ar fi nesigură, de exemplu o autentificare sau parola de conectare. Explicațiile se află în baloanele (i) de lângă fiecare control.

Apasă `/` sau Ctrl+K oriunde în setări ca să le cauți pe toate, inclusiv după numele pe care le aveau paginile în versiunile anterioare. Ordinea plăcilor poate fi schimbată la **Aspect**. Acolo unde o coloană de plăci lângă pagină ar strânge-o, ca într-o fereastră lată de circa 1200 de pixeli sau pe un telefon, plăcile devin o bandă deasupra paginii, care derulează lateral.

![Setări cu placa General deschisă.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Setări cu placa General deschisă.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Această pagină listează fiecare placă și cardurile ei și trimite la pagina care explică fiecare card în detaliu.

## General {#general}

- **Secțiuni** pornește sau oprește fiecare secțiune: **Containere**, **VM-uri**, **Flash**, **Foldere**, **Seturi de date ZFS** și **Auto-backup**. O secțiune oprită nu este salvată de niciun program, nici de planul propriu al unui element, iar elementele ei nu se numără. **Copiază acum** funcționează în continuare. **Flash** apare doar pe Unraid.
- **Containere** conține ce se aplică tuturor containerelor, de exemplu **Copii SQL automate ale bazelor de date**. Tot ce afectează un singur container se află pe pagina lui. Vezi [Dumpuri de baze de date](backups.md#database-dumps).

## Aspect {#look}

- **Aspect**: **Limbă** (42 de limbi), **Temă** (**Întunecat**, **Luminos** sau **Sistem**), **Colțuri**, **Animație** (**Oprit**, **Discretă**, **Sălbatică** sau **Furtunoasă**), **Notificări discrete**, care ascunde mesajele de succes, cum sunt confirmările de salvare, și **Ordinea paginilor de setări**, unde ții apăsată o placă și o tragi la locul ei sau folosești Alt și tastele săgeți.
- **Etichete**: cât din un control este afișat, separat pentru **Butoane**, **Bară laterală**, **File** și **Bară inferioară**: **Text**, **Text și simbol**, **Simbol** sau **Reactiv**, care arată doar simbolul și face să alunece cuvintele când treci cu mouse-ul peste el. **Bară laterală** afectează doar aspectul de desktop, iar **Bară inferioară** doar aspectul de telefon.
- **Culori**: o singură **Culoare de accent** sau **Mod curcubeu**, în care fiecare rând al unei liste primește propria culoare dintr-o paletă de opt. Modul curcubeu adaugă **Mod disco**, în care culorile alunecă lent spre următoarea, **Mod reactiv**, în care culoarea unui rând se vede doar cât treci peste el, rulează sau este selectat, **Rotația culorilor** și **Paleta de culori** însăși.

BombVault păstrează aspectul pe server. Un browser care vine pentru prima dată îl preia. Când sistemul tău cere mai puțină mișcare, asta câștigă întotdeauna în fața **Animației**.

## Programare {#schedule}

**Săptămâna**, **Backup total** cu **Când rulează backupurile**, **Recuperează backupurile ratate după pornire** și **Comenzi înainte și după**, **Programări pe secțiuni** cu **Copii externe pe secțiuni**, **Elemente programate**, **Ordine** cu **Repornire după backup**, **Instantaneu în loc de oprire**, **Actualizări după backup** și **Inactivitate înainte de copie**. Vezi [Programări](schedules.md).

## Destinații {#storage-locations}

- **Arată și în bara laterală** fixează pagina Destinații în bara laterală.
- Lista destinațiilor tale, cu **Adaugă destinația**. Un clic deschide pagina unei destinații, vezi [Pagina unei destinații](offsite-recovery.md#location-page).
- **Autentificări**: autentificările salvate, cu steaua implicită, vezi [Autentificări](offsite-recovery.md#sign-ins).
- **Destinații pe secțiuni**: fluxul 3-2-1, în care fiecare rând este un comutator, vezi [Implicite pe secțiuni](offsite-recovery.md#placement-defaults).
- **Depozite**: depozite create pentru elemente individuale, vezi [Amplasare per element](offsite-recovery.md#placement).
- **Cât timp se păstrează** trimite la destinații, unde se setează retenția, iar **Previzualizare** arată **Ce ar șterge următoarea rulare**, pe depozit și pe element, înainte să se întâmple. Nu schimbă nimic și nu blochează nimic, așa că răspunde chiar și în timpul unui backup.
- **Copii externe**: un rând pe secțiune cu **Testează conexiunea**, **Replică acum** și **Configurează** sau **Editează**, vezi [Copii off-site](offsite-recovery.md#off-site-copies).
- **Lățime de bandă externă**: **Limită încărcare (KiB/s)** și **Limită descărcare (KiB/s)** pentru fiecare copie. 0 înseamnă nelimitat.

Cardul **Streaming**, care coboară limita de încărcare cât timp Plex, Jellyfin sau Emby fac streaming, se află chiar pe pagina Destinații.

## Restaurare {#restore}

- Un link către pagina **Restaurare**.
- **Foldere de restaurare**: **Folder de restaurare implicit** pentru o restaurare într-un folder, cu **Subfolder pentru fiecare secțiune** și o **Excepție** pe secțiune. Vezi [Unde ajunge o restaurare într-un folder](restore.md#folders).
- **Registre de containere**: autentificări pentru registre private, de exemplu o imagine `ghcr.io` rezervată sponsorilor. O restaurare le folosește ca să tragă o imagine lipsă, iar o actualizare după backup ca să verifice dacă există una mai nouă. Imaginile publice sunt trase fără ele. **Adaugă registru** oferă registre publice, găzduite propriu, ale unui furnizor cloud și **Alt registru** pentru orice adresă.
- **Pentru urgențe**: **Descarcă kitul de recuperare**, care cere o parolă de conectare. Vezi [Kit de recuperare](offsite-recovery.md#recovery-kit).

## Notificări {#notifications}

**Notificări** cu **Notifică**, rezumatele, **Raportează backupurile întârziate**, **Notificări Unraid** și **Rezumat săptămânal**, **Canale** cu Webhook, Apprise, Matrix, Gotify și E-mail (SMTP), plus **Healthchecks**. Vezi [Notificări](notifications.md).

## Verificare {#verify}

- **Verifică copiile de rezervă**: rezultatul ultimului `restic check` pe secțiune, cu **Deblochează** și **Curăță** sub **Mai mult** în fiecare rând. **Deblochează** șterge o blocare rămasă, iar **Curăță** aplică la cerere regulile de păstrare.
- **Verificări automate**: verificarea de restaurare programată cu **Eșantion de date (%)** și țintele de test, **Test de pornire**, care restaurează și un container într-o copie izolată și îl pornește, **Test DR extern programat** și **Verifică protecția la ștergere**, testul programat de manipulare. Vezi [Teste DR](offsite-recovery.md#dr-drills) și [Test de manipulare](offsite-recovery.md#tamper-test).
- **Anomalii**: detectarea, sensibilitatea și ce constatări trimit un mesaj. Vezi [Anomalii](anomalies.md).

## Securitate {#security}

- **Parolă**: parola de conectare, de cel puțin 12 caractere. Schimbarea sau ștergerea ei cere parola curentă și, cu autentificarea în doi pași pornită, codul.
- **Autentificare în doi pași**: un cod dintr-o aplicație de autentificare peste parolă, cu opt coduri de recuperare afișate o singură dată.
- **Chei de acces**: te conectezi cu o cheie de pe telefon, laptop sau cheie de securitate. Au nevoie de un nume de domeniu real și de un certificat în care browserul are încredere.
- **Aplicația Android**: **Telefoanele se pot împerechea cu cele douăsprezece cuvinte ale grupului**, ceea ce permite unui telefon împerecheat să se conecteze fără parolă.
- **Criptare**: **Criptează copiile de rezervă**, pornit implicit.

Mai multe lucruri cer o parolă de conectare, de exemplu kitul de recuperare, depunerile de chei și ștergerea definitivă a backupurilor. Vezi [Model de securitate](configuration.md#security-model).

## Instanțe {#instances}

- **Arată și în bara laterală** fixează pagina Instanțe în bara laterală.
- **Mai multe servere** pornește părțile unui grup: **Instanțe**, pagina cu un card pentru fiecare membru, **Receptor** și **Preluare**. Vezi [Instanțe și roluri](offsite-recovery.md#roles).
- Un link către pagina Instanțe, cu părțile care sunt pornite.

## Împerechere {#pairing}

**Împerechere** cu **Generează frază** sau **Introdu frază**, cardurile cu pași, numele acestei instanțe și **Adrese cunoscute**, cardul **Relay** cu cele patru forme ale lui și **Ce vede relay-ul**, membrii grupului și **Server ZFS fără BombVault** pentru o țintă de replică prin SSH. Vezi [Împerecherea instanțelor](offsite-recovery.md#pairing) și [Trimitere către un server ZFS](zfs-datasets.md#replica-push).

## Integrări {#integrations}

- **Server MCP**: chei pentru asistenți IA, **Alege un client** și jurnalul fiecărei chei. Vezi [Server MCP](mcp.md).
- **Tokenuri API**: tokenuri cu nume pentru scripturi și dashboarduri. Vezi [Tokenuri](api.md#tokens).
- **Servicii**: **Home Assistant** prin MQTT, **Găsește în rețea** (mDNS ca `bombvault.local`) și **Expune /metrics** pentru Prometheus, cu un **Token de scrape (opțional)**. Vezi [Home Assistant](api.md#home-assistant) și [Găsește BombVault în rețea](api.md#mdns).
- **Widget pentru dashboard**: un token doar pentru citire și o adresă pentru orice dashboard care afișează un iframe, cum sunt Homepage sau Heimdall, cu doar jurnalul de activitate în direct. Vezi [Widget pentru dashboard](api.md#dashboard-widget).
- **SSH al gazdei**: cheia publică a BombVault și **Testează conexiunea**, pentru backupurile VM și ale seturilor de date ZFS și pentru notificările Unraid. Vezi [ghidul de backup VM](vm-backup-ssh-setup.md).

## Aplicații {#apps}

- **Aplicația Android**: APK-ul pentru versiunea pe care o rulează serverul tău, cu un cod QR, și lista F-Droid. Vezi [Aplicația Android](android.md#install).
- **ParleyPort**: containerul relay cu șablonul lui Unraid și comanda Docker care îl pornește.
- **BombVault Widget**: o placă pe Dashboardul Unraid cu jurnalul de activitate al BombVault și următoarea rulare. Cu o conexiune SSH la gazdă, cardul instalează și elimină singur pluginul. Fără ea îți dă adresa `.plg` de instalat la **Plugins, Install Plugin**.

## Sistem {#system}

- **Exportă / importă setările**: întreaga ta configurație ca fișier JSON portabil, cu sau fără credențiale. Importul arată mai întâi **Acest fișier conține** și omite legăturile către destinații care nu mai există. **Criptează exporturile cu age** sigilează exporturile simple, ZIP-ul flash și kitul de recuperare pentru **Destinatari age** pe care îi listezi. Vezi [Setări portabile](configuration.md#portable-settings-export-and-import).
- **Performanță**: **Fire CPU (0 = toate)**, **Limita dimensiunii cache-ului (MB, 0 = nelimitat)** pentru cache-ul restic de sub `/config` și **Limită de memorie per copie (MiB, 0 = fără)**, o limită flexibilă care face un backup mare să încetinească în loc să fie oprit pentru că a rămas fără memorie.

## Info {#info}

- **Versiune**: BombVault, GlimStone, restic și rclone, cu **Caută actualizări** și **Nou**, care numără schimbările pe care nu le-ai văzut încă și deschide o fereastră care le listează.
- **Ajutor**: **Manual** deschide această documentație, iar **Raport de eroare** descarcă un ZIP cu date șterse pentru un issue GitHub. Cere o parolă de conectare.
- **Despre BombVault**: proiectul, licența lui și cum îl poți contacta pe autor.
