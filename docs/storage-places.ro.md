# Locuri de stocare

Un loc de stocare este un loc în care BombVault păstrează copii de siguranță: un folder pe acest Unraid, o partajare pe un NAS, un bucket la un furnizor de cloud, un rest-server, un cont SFTP sau un Nextcloud. Conectezi fiecare loc o singură dată, în **Setări, Stocare**, iar datele lui de acces, retenția, protecția și locația în care se află țin de el. Domeniile (containere, VM-uri, flash-ul, configurația proprie a BombVault, seturile de fișiere și seturile de date ZFS) aleg apoi dintre locuri: unde este stocat fiecare domeniu și unde este copiat. Seturile de date ZFS au rândul lor pe cardul Domenii cât timp domeniul ZFS este pornit.

## Adăugarea unui loc {#add-a-place}

**Adaugă loc** deschide o fereastră cu câte o casetă pentru fiecare furnizor, în trei grupuri: stocare în cloud, servicii găzduite de tine, precum și NAS-uri și acest server.

1. Alege o casetă și completează formularul ei. Butonul cu ochi arată un secret pe care l-ai tastat.
2. **Testează conexiunea** verifică locul și nu creează nimic. Pentru folderul fiecărui domeniu spune ce a găsit: gol sau încă inexistent, conține deja un depozit restic, sau eroarea care l-a oprit.
3. Dă un nume locului; numele furnizorului este deja completat. Pentru un dispozitiv pe care îl administrezi tu, răspunde la **Unde se află dispozitivul?**. Furnizorii de cloud sunt întotdeauna într-o altă locație, iar un folder pe acest Unraid este întotdeauna aici.
4. **Adaugă** salvează locul.

Un loc nou nu este folosit încă de niciun domeniu. Alege-l la **Stocat în** sau **Copiat în** pe [cardul Domenii](#domains), sau pe cardul unui element, doar pentru acel element.

## Foldere {#folders}

Un loc păstrează câte un folder pentru fiecare domeniu: `container`, `vms`, `flash`, `config`, `files` și `zfs`, aceleași nume pe care le folosesc căile implicite de backup. Folderele sunt listate în detaliile locului și pot fi redenumite acolo (vezi [Schimbarea unei adrese](#addresses)). Un domeniu fără folder într-un loc nu poate alege acel loc.

Când un domeniu folosește un loc în ambele roluri, al doilea rol primește un sufix, iar primul își păstrează folderul. Un loc care primește deja copiile unui domeniu stochează elementele trimise direct către el în `<folder>-direct`; un loc care stochează deja un domeniu îi primește copiile în `<folder>-copies`.

Unele locuri sunt ele însele un depozit restic: o adresă care conținea deja un depozit când locul a fost adăugat, un depozit cu nume dintr-o configurație existentă sau o țintă de copiere aflată la rădăcina unui bucket. Un astfel de loc nu are foldere, toate domeniile folosesc în comun singurul lui depozit și nu preia un al doilea rol. Ca să stochezi mai mult la același furnizor, conectează un alt bucket sau folder ca loc separat.

## Detaliile locului {#details}

Fiecare loc este un rând cu furnizorul lui, cu ce este folosit și cu ultimul test sau ultima copiere. **Testează** verifică fiecare adresă din acel loc, iar **Detalii** îi deschide setările. Fiecare modificare din detalii se salvează imediat ce o faci.

- **General**: numele, comutatorul care pornește și oprește locul, adresa și, pentru un dispozitiv pe care îl administrezi tu, **Unde se află dispozitivul?** (vezi [În afara sediului](#off-the-premises)).
- **Retenție**: keep-last, zilnic, săptămânal și lunar, pentru fiecare depozit din acel loc. Un loc nou pornește cu regulile implicite; un loc cu toate regulile la zero nu curăță niciodată nimic.
- **Protecție**: comutatorul **Append-only**. Partea îndepărtată trebuie să impună append-only; cu comutatorul pornit, BombVault nu curăță și nu șterge niciodată nimic acolo. La un rest-server cu append-only pornit, **Testează append-only** rulează testul de manipulare pe fiecare cale de domeniu, fiecare copie pornită și fiecare depozit din acel loc și arată un singur răspuns pentru tot locul, *ștergeri refuzate* sau *ștergeri acceptate* (vezi [Off-site și recuperare](offsite-recovery.md)). Doar locurile la distanță au această secțiune, pentru că nimic de pe această mașină nu poate împiedica ștergerea unui depozit local.
- **Acces**: datele de acces și, pentru S3, clasa de stocare. Un loc care folosește datele de acces comune primește un set propriu la prima modificare. Un depozit direct din acel loc pe care noile date de acces nu îl pot deschide le păstrează pe cele vechi, iar răspunsul spune asta. Locurile de tip folder, SFTP și rclone nu au această secțiune.
- **Limite**: rata de încărcare și de descărcare și bugetul de creștere.
- **Foldere**: câte un comutator pentru fiecare domeniu, cu numele folderului lui. Un domeniu oprit aici nu poate alege locul.

Scăderea retenției întreabă mai întâi și spune câte elemente sunt afectate; oprirea append-only întreabă mai întâi și spune câte depozite din acel loc pierd protecția. Oprirea unui loc oprește fiecare depozit din el; un loc în care este stocat un domeniu nu poate fi oprit.

## Cardul Domenii {#domains}

Cardul are câte un rând pentru fiecare domeniu, cu programarea lui, locul în care este stocat, locurile în care este copiat și excepțiile lui.

- **Stocat în**: cât timp calea de backup a domeniului nu conține copii de siguranță, locul ales devine locul de stocare al domeniului, iar calea se mută acolo. Odată ce conține copii de siguranță, alegerea pentru containere, VM-uri și seturi de fișiere devine valoarea implicită pentru elementele noi, care o preiau la prima lor copie de siguranță; elementele care au deja copii de siguranță rămân unde sunt, pentru că BombVault nu mută niciodată o copie de siguranță. Pentru flash și configurația proprie a BombVault se mută locul de stocare, iar copiile de siguranță deja scrise rămân în vechiul loc.
- **Copiat în**: câte un chip pentru fiecare loc care poate primi copiile domeniului. Bifarea unui chip face din acel loc o țintă de copiere a domeniului; prima dată, BombVault spune dinainte câte elemente și instantanee și câte date trimite prima rulare. Debifarea oprește copiile noi: copiile aflate deja acolo rămân și îmbătrânesc după retenția locului, iar elementele cu alegere proprie continuă să copieze acolo. Debifarea ultimului chip oprește toate copiile, inclusiv către locurile adăugate mai târziu, până când este bifat din nou unul. Un loc oprit apare ca un chip estompat și nu poate fi ales.
- **Excepții**: elementele cu alegere proprie, ca listă cu legături către cardurile lor.
- **Copiază acum** rulează imediat copiile domeniului.

Un domeniu suspendat după o reconstrucție prin Descoperă arată suspendarea pe rândul lui, cu **Confirmă valoarea implicită** (vezi [Amplasare per element](offsite-recovery.md#placement)).

## Schimbarea unei adrese {#addresses}

Folderul unui domeniu poate fi schimbat în detaliile locului, la fel și adresa unui loc local, de exemplu după ce un depozit a fost mutat manual pe alt disc. BombVault testează fiecare adresă afectată de schimbare și o acceptă când fiecare adresă nouă este goală și la cea veche nu era stocat nimic, sau când fiecare adresă nouă conține același depozit restic ca cea veche. Orice altceva este refuzat, cu numărul de copii de siguranță aflate încă la vechea adresă. Un loc la distanță își păstrează adresa; ca să salvezi în altă parte, conectează noua destinație ca loc separat.

BombVault construiește lista locurilor din propria bază de date și nu listează niciodată un depozit la distanță ca să o completeze; testul rulează doar când schimbi ceva.

## Eliminarea unui loc {#remove}

Un loc poate fi eliminat doar cât timp nu îl folosește nimic: niciun domeniu nu este stocat acolo, nicio valoare implicită nu indică spre el, niciun element nu este stocat acolo și niciun depozit direct din el nu conține elemente. Altfel, refuzul enumeră ce îl ține ocupat. Eliminarea ia cu ea țintele lui de copiere și datele lui de acces proprii, dacă nu le folosește o sursă de preluare sau un alt loc. În stocarea propriu-zisă nu se șterge nimic, iar confirmarea spune câte copii rămân acolo.

## Fără loc {#without-a-place}

O adresă care nu se potrivește cu forma unui loc plus un folder funcționează în continuare și apare la **Fără loc**, cu adresa ei. Printre ele se află adresele native `b2:`, `gs:` și `swift:`. **Atribuie unui loc** leagă un astfel de rând de un loc, după același test ca la [schimbarea unei adrese](#addresses). O țintă de copiere fără loc apare și pe rândul domeniului ei, lângă chipuri, și continuă să copieze. Un rând la distanță de acolo are propriul comutator **Append-only**, iar oprirea lui întreabă mai întâi, cu numărul de elemente care păstrează backupuri la acea adresă. Un depozit direct urmează comutatorul țintei sale.

## În afara sediului {#off-the-premises}

**Unde se află dispozitivul?** are două răspunsuri: **Aici, în casă** și **Într-o altă locație**. O copie contează ca locație separată, pentru rândul 3-2-1 de pe carduri și pentru verificările off-site de pe panoul principal, doar când locul ei se află într-o altă locație. Un al doilea disc sau un NAS în aceeași casă este o a doua copie, nu o a doua locație. Răspunsul nu schimbă nicio copie. Furnizorii de cloud sunt întotdeauna într-o altă locație, iar un folder pe acest Unraid este întotdeauna aici, așa că formularul nu întreabă pentru ei; pentru orice alt loc, schimbă răspunsul în detaliile lui. Un loc aflat într-o altă locație poartă pe rândul lui marcajul **Altă locație**.

## Tipuri de conexiune

### Folder pe acest Unraid sau pe un NAS {#kind-local}

Adresa este o cale sub `/mnt`, scrisă fără `/mnt`, de exemplu `user/bombvault`, iar folderul fiecărui domeniu se află sub ea: `user/bombvault/container`.

- **Folder pe acest Unraid** alege dintre partajări, discuri și pool-uri.
- **Synology**, **QNAP**, **TrueNAS**, **Alt Unraid** și **Altă partajare** aleg din `/mnt/remotes`. Montează mai întâi partajarea în Unraid, de exemplu cu pluginul Unassigned Devices. Host Data trebuie montat Read/Write - Slave, altfel o partajare care se montează după pornirea BombVault rămâne invizibilă până la o repornire (vezi [Configurare](configuration.md)).

Selectorul de foldere creează un folder acolo unde te afli, cu **Folder nou**. Testul verifică dacă folderul este gol sau inexistent și dacă BombVault poate scrie acolo.

### S3 {#kind-s3}

Adresa este `s3:https://<endpoint>/<bucket>/<path>`, de exemplu `s3:https://s3.eu-central-003.backblazeb2.com/tower-backups/bombvault`.

- **Backblaze B2** are nevoie doar de ID-ul cheii și de cheia aplicației. BombVault întreabă B2 la ce bucket, endpoint S3 și folder este limitată cheia și construiește adresa din ele. O cheie care are acces la toate bucket-urile îți oferă bucket-urile spre alegere.
- **Amazon S3**, **Cloudflare R2**, **Wasabi**, **Hetzner Object Storage**, **Storj**, **IDrive e2**, **Scaleway**, **OVHcloud**, **DigitalOcean Spaces**, **IONOS**, **Contabo**, **Exoscale** și **Vultr** cer cheia și, acolo unde furnizorul are nevoie, regiunea, ID-ul contului sau endpointul. BombVault completează endpointul și listează bucket-urile când cheia are voie să le listeze; altfel tastează numele bucket-ului.
- **Google Cloud Storage** trece prin interfața sa S3 cu o cheie HMAC, creată în setările Cloud Storage, la Interoperability. Un fișier de cont de serviciu nu funcționează aici.
- **MinIO**, **SeaweedFS**, **Garage**, **Ceph**, **JuiceFS**, **RustFS**, **Versity S3 Gateway** și **Alt serviciu S3** cer adresa serviciului și o cheie.

Clasa de stocare se setează în detaliile locului, limitată la nivelurile pe care o restaurare le poate citi fără dezghețare.

### rest-server {#kind-rest}

Adresa este `rest:<url>/<user>`, de exemplu `rest:https://nas.lan:8000/tower`. Formularul cere adresa serverului, un utilizator și o parolă. Cu `--private-repos`, un utilizator poate ajunge doar la căile care încep cu propriul lui nume, așa că BombVault pune utilizatorul primul, dacă nu tastezi o altă cale. Când serverul refuză o cale din afara celei a utilizatorului, eroarea spune asta.

Formularul rest-server conține o rețetă gata de lipit pentru un rest-server în mod append-only, cu un utilizator pentru acest BombVault. **Arată rețeta** generează o parolă, afișată o singură dată, și oferă o linie `docker run`, un fișier compose și un șablon Unraid, fiecare cu linia `htpasswd` care trebuie pusă pe server; utilizatorul și parola ajung direct în formular.

**Alt BombVault** listează deasupra propriilor câmpuri ofertele deschise pe care alte instanțe le-au trimis prin Flotă. Acceptarea uneia adaugă un loc care păstrează doar copiile domeniului oferit, pentru că o ofertă aduce un utilizator doar pentru acel domeniu. Acceptarea pe pagina Flotă adaugă același loc.

### SFTP {#kind-sftp}

Adresa este `sftp://<user>@<host>:<port>/<path>`, de exemplu `sftp://bv@backup.lan:22/bombvault`. Formularul cere gazda, portul și utilizatorul și arată cheia publică a BombVault. Adaugă această cheie în `~/.ssh/authorized_keys` al utilizatorului de pe server; acolo nu trebuie instalat nimic altceva. BombVault acceptă cheia de gazdă a serverului la primul contact și o verifică de atunci încolo.

**Hetzner Storage Box** completează `<user>.your-storagebox.de` și portul 23. Instalează cheia pe Storage Box cu comanda Hetzner, care cere o singură dată parola acestuia:

```sh
echo '<public key>' | ssh -p 23 <user>@<user>.your-storagebox.de install-ssh-key
```

### WebDAV: Nextcloud, ownCloud, OpenCloud {#kind-webdav}

Formularul cere adresa serverului, utilizatorul și o parolă de aplicație. Creează parola de aplicație în setările de securitate ale contului și introdu ID-ul utilizatorului, nu o adresă de e-mail. BombVault construiește calea WebDAV pe care o folosește produsul și îi predă conexiunea lui restic prin variabilele de mediu ale rclone, cu parola în forma obscurizată de rclone. Adresa arată ca `rclone:bvp<id>:<path>`, unde `bvp<id>` este un remote care există doar în acel mediu; în configurația rclone nu se scrie nimic.

### Azure Blob {#kind-azure}

Adresa este `azure:<container>:/<path>`. Formularul cere contul de stocare și cheia lui de acces; după **Testează conexiunea** listează containerele contului spre alegere, sau tastezi numele unui container. BombVault transmite contul și cheia către restic ca `AZURE_ACCOUNT_NAME` și `AZURE_ACCOUNT_KEY`.

### rclone {#kind-rclone}

Adresa este `rclone:<remote>:<path>`. Formularul listează spre alegere remote-urile din configurația rclone a BombVault. Ca să înlocuiești acea configurație, lipește un `rclone.conf` întreg la **Configurație rclone** și apasă **Salvează configurația**. Configurația se salvează imediat și servește fiecărui loc rclone, indiferent dacă fereastra adaugă apoi un loc sau nu.

## Copii între locuri cu date de acces diferite {#different-credentials}

Un domeniu stocat într-un loc la distanță este sursa copiilor lui. `restic copy` rulează cu un singur mediu, iar BombVault adaugă datele de acces ale sursei la cele ale țintei când cele două nu setează aceeași variabilă la valori diferite. Un loc Nextcloud și un loc B2 folosesc variabile diferite, așa că un domeniu stocat în Nextcloud poate fi copiat în B2. Două conturi S3 sau doi utilizatori de rest-server ar avea nevoie de aceleași variabile cu valori diferite; restic nu le poate primi pe amândouă, iar chipul de pe cardul Domenii spune că datele de acces nu se potrivesc.
