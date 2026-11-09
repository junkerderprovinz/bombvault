# Off-site și recuperare

!!! note "Copiile off-site așteaptă după o reconstruire"
    Când Asistentul de urgență sau **Descoperă copii de rezervă** reconstruiește elemente fără setările vechi, copiile off-site ale acelor secțiuni așteaptă până când preiei copiile lor ca fiind ale acestui server. Vezi [După o reconstruire](#after-a-rebuild).

Copiile locale te protejează de un container pierdut sau de o actualizare proastă. Copiile off-site și un kit de recuperare testat te protejează de pierderea întregii stații, de ransomware sau de un incendiu. Această pagină acoperă destinațiile și copiile off-site, cum faci o copie rezistentă la manipulare, cum dovedești că poți restaura, cum împerechezi mai multe servere și cum aduci totul înapoi când BombVault însuși a dispărut.

## Copii off-site {#off-site-copies}

Păstrează copia locală rapidă și adaugă una sau mai multe destinații în afara locuinței. BombVault copiază acolo copiile de rezervă noi cu `restic copy`, în regim best-effort, așa că un sughiț off-site nu face niciodată să eșueze copia locală. În această formă fiecare secțiune scrie mai întâi în dosarul ei propriu, iar depozitul off-site este o copie. Dosarul propriu nu trebuie să fie pe acest server: vezi [Dosarul propriu al unei secțiuni](#own-folder) pentru cazul în care îl ții într-o destinație, de exemplu S3 sau un rest-server.

- **Mai multe destinații pe secțiune.** Fiecare secțiune (containere, VM-uri, flash, Auto-backup, foldere și seturi de date ZFS) poate copia simultan în mai multe destinații off-site, așa că poți ține, de exemplu, un rest-server pe stația unui prieten și un bucket S3 în paralel. Cardul **Destinații pe secțiuni** de pe pagina Destinații pornește sau oprește fiecare linie de la o secțiune la o destinație.
- **Când rulează copiile.** Cardul **Copiază** al unei destinații stabilește **Când se copiază**: **După fiecare copie de rezervă** sau după un plan propriu, care adună copiile de rezervă și copiază la ore fixe. La **Setări, Programare, Copii externe pe secțiuni**, **Excepție** dă unei secțiuni un plan propriu de copiere.
- **Cardul Copii externe.** La **Setări, Destinații** fiecare secțiune cu o copie off-site are un rând în **Copii externe**, cu **Testează conexiunea**, **Replicează acum** pentru a porni o copiere la cerere și **Configurează** sau **Editează**. Restul setărilor de copiere ale secțiunii se deschid de acolo, în fereastra ei.
- **Retenția** se stabilește pe fiecare destinație la **Cât timp se păstrează**, așa că poți ține copiile off-site mai mult, ca arhivă. Copia din câmpul off-site al unei secțiuni o urmează ca orice altă copie, iar o secțiune își poate păstra valorile ei la **Reguli pe secțiuni**. Dacă toate valorile sunt zero, copiile off-site nu se taie niciodată. Vezi [Cât timp se păstrează](#keep).
- **Limitele de lățime de bandă** (**Lățime de bandă externă** la **Setări, Destinații**, sau cardul **Copiază** al unei destinații) plafonează rata de încărcare și descărcare a restic, ca să nu satureze copiile WAN-ul tău. Se introduc în KiB/s, unitatea pe care o ia restic, iar fiecare câmp arată aceeași rată în MB/s și Mbit/s în timp ce tastezi: 2048 se citește ca aproximativ 2,0 MB/s (16,8 Mbit/s).
- **Comprimarea** este **Oprit**, **Automat** (implicit în restic) sau **Maxim**, stabilită pe fiecare destinație și, pentru o singură secțiune, la regulile ei de acolo. Copiile de rezervă, copierile și curățarea scriu cu ea, iar kitul de recuperare o numește, așa că restic simplu poate continua să scrie la fel.
- **Streamingul are prioritate.** Cât timp un server media precum Plex, Jellyfin sau Emby transmite, copiile se încarcă la o limită mai mică și revin la cea obișnuită la câteva minute după ce se oprește transmisia. Cardul **Streaming** de pe pagina Destinații are **Încetinește copiile externe în timpul streamingului**, **Servere media** (containere cu Plex, Jellyfin sau Emby în numele imaginii, până alegi tu), **Streaming peste (Mbit/s)**, **Limită încărcare în timpul streamingului (KiB/s)** și **Revenire la normal după (min)**. BombVault citește traficul din Docker, așa că un server media aflat în rețeaua gazdei nu poate fi măsurat. REST, S3, B2, Azure, Google Cloud, Swift și rclone prin HTTP încetinesc chiar în mijlocul unei copieri. SFTP și folderele locale sau montate preiau limita mai mică la pasul următor al copierii.
- **Stocare rece (S3).** O destinație S3 poate folosi o **Clasă de stocare** mai ieftină, pe care restic o poate citi totuși imediat, precum Standard-IA, One Zone-IA, Intelligent-Tiering sau Glacier Instant Retrieval. Glacier Flexible Retrieval și Deep Archive lipsesc, pentru că restic nu poate restaura din ele. Un remote rclone își stabilește clasa în configurația rclone.
- **Conexiuni simultane.** Pe o destinație S3, B2, Azure, Google Cloud, Swift, rest-server, SFTP sau rclone, **Conexiuni simultane** din cardul **Conexiune** stabilește câte conexiuni deschide restic deodată, de la 1 la 32. Dacă rămâne gol, restic își păstrează valoarea implicită de 5. Mai multe pot grăbi o linie îndepărtată sau lentă, mai puține menajează un furnizor care limitează viteza. Copiile de rezervă, copierile, verificările, curățarea și restaurările de pe destinație îl folosesc toate, iar un export al setărilor îl include. Un folder local sau montat nu are această setare.
- **Buget de creștere.** **Buget de creștere (GB, 0 = oprit)** din fereastra de copiere a unei secțiuni dă alarmă când depozitul crește peste acea dimensiune. Nu șterge niciodată nimic, deci se potrivește unei copii append-only din care nimic nu se curăță de aici.
- Cât timp rulează o copiere, antetul de stare din prezentare spune **Copierea off-site rulează**. Este un indicator de activitate, nu o bară de procente, pentru că `restic copy` nu expune progres citibil automat.

!!! note "Restaurează din orice loc"
    Fiecare element își listează copiile ca o singură axă a timpului peste toate locurile în care se află o copie, în fila lui **Copii de rezervă**. O copie trimisă la B2 apare o dată, marcată cu fiecare loc care o deține. O restaurare ia primul loc pe care îl poate atinge, începând cu depozitul în care este scris elementul, și poți alege alt loc pentru fiecare copie de rezervă. Destinațiile off-site sunt citite doar când apeși **Verifică**, pentru că listarea stocării cloud durează. Ștergerea dintr-un loc verifică întâi celelalte locuri și spune dacă era ultima copie. Vezi [Restaurare](restore.md).

## Destinații {#destinations}

O destinație este un loc unde ajung copiile de rezervă: un folder local, o partajare, un server al tău sau un serviciu cloud. Se configurează o singură dată și servește apoi fiecare secțiune. Deschide **Destinații** din placa ei de setări sau pornește acolo **Arată și în bara laterală** ca să o fixezi.

![Pagina Destinații: lista destinațiilor și fluxul care arată ce secțiune merge unde.](assets/screenshots/storage/locations-dark.png#only-dark)
![Pagina Destinații: lista destinațiilor și fluxul care arată ce secțiune merge unde.](assets/screenshots/storage/locations-light.png#only-light)

**Adaugă destinație** deschide o fereastră în patru pași:

1. **Furnizor.** Fiecare serviciu este listat cu logoul lui, cu o căutare în colțul din dreapta sus al ferestrei, pe grupe: **Local** pentru un disc sau o partajare atașată la acest server, **NAS sau server** (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV și altele), **Serviciu S3** (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 și altele), propriul tău server S3 (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), **Stocare în cloud** (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud și restul pe care le acceptă rclone) și **Din grupul tău**, adică serverele receptoare ale grupului tău. Fiecare spune cât de bine se potrivește pentru copii de rezervă: unitățile cloud încetinesc la multe cereri, așa că prima copie și curățarea durează mai mult acolo. Pentru un serviciu pe care nu îl acoperă nicio placă, placa **Rclone** de la **Configurație rclone proprie** primește remote-uri lipite dintr-un rclone.conf.
2. **Autentificare.** Câmpurile depind de serviciu: o cheie de acces pentru S3, un utilizator și o parolă pentru WebDAV și SMB, o parolă de aplicație acolo unde autentificarea în doi pași o blochează pe cea obișnuită, cheia publică SSH a BombVault pentru SFTP și Storage Box sau un token pentru serviciile care se autentifică printr-un browser. Pentru acestea, fereastra arată o comandă `rclone authorize` de rulat pe un calculator cu browser, iar tokenul pe care îl tipărește merge în câmp. Dacă ai deja o [autentificare salvată](#sign-ins) pentru serviciu, **Autentificare salvată** o alege în locul **Chei noi**. **Testează conexiunea** verifică autentificarea înainte să se salveze ceva. Fereastra listează apoi folderele de pe destinație, cu **Folder nou** pentru a crea unul și cu spațiul liber acolo unde serviciul îl raportează. Un folder gol este cel mai sigur. BombVault pune în el câte un folder pe secțiune.
3. **Protecție.** Fereastra spune clar ce poate face serviciul. Un rest-server în mod append-only refuză ștergerea, iar testul de manipulare verifică asta. Un bucket S3 poate păstra versiuni vechi prin versionare și object lock, ceea ce BombVault nu poate verifica încă. O unitate cloud nu poate refuza deloc ștergerea: cine ajunge în server ajunge și la acea copie. Pornește **Protecție împotriva ștergerii (append-only)** doar acolo unde partea îndepărtată chiar refuză ștergerea. BombVault nu mai curăță atunci niciodată acolo. **Pentru o urgență** spune ce conține kitul de recuperare pentru această destinație.
4. **Pentru ce secțiuni.** Alege secțiunile care se salvează aici. Prima copiere aduce tot istoricul. Elementele care și-au ales propriile destinații nu o primesc singure pe cea nouă, iar fereastra le numește.

![Adaugă destinație: primul pas, cu fiecare furnizor ca o placă.](assets/screenshots/storage/add-providers-dark.png#only-dark)
![Adaugă destinație: primul pas, cu fiecare furnizor ca o placă.](assets/screenshots/storage/add-providers-light.png#only-light)

![Adaugă destinație: pasul de autentificare după un test reușit.](assets/screenshots/storage/add-signin-dark.png#only-dark)
![Adaugă destinație: pasul de autentificare după un test reușit.](assets/screenshots/storage/add-signin-light.png#only-light)

Unele servicii cer ceva în plus la autentificare:

- **SFTP și Storage Box.** **Adaugă cheia** pune cheia publică a BombVault pe server cu **Parola contului**, care este folosită pentru acea singură autentificare și nu este păstrată nicăieri. Dedesubt apare comanda pentru a face asta manual. Testul arată **Cheia serverului** cu amprenta ei. BombVault o reține și refuză serverul dacă mai târziu arată alta.
- **Cheile S3 pentru un singur bucket**, cum ar fi un token de obiect Cloudflare R2 sau o cheie AWS restrânsă, s-ar putea să nu aibă voie să listeze celelalte bucket-uri. Tastează bucket-ul la **Bucket**, iar testul și lista de foldere deschid acel bucket.
- **Regiuni și adrese S3.** Regiunea se citește dintr-un endpoint care o numește (AWS, Backblaze B2, Hetzner, Scaleway, OVH, Wasabi), Garage pornește cu propria regiune implicită, iar IDrive e2 cere endpoint-ul pe care îl arată tabloul lui de bord. O adresă lipită cu tot cu bucket, așa cum o arată tabloul de bord R2, completează serverul și bucket-ul. Un bucket creat din navigatorul de foldere ajunge în regiunea numită în formular.
- **Clasa de stocare.** Selectorul oferă doar clasele pe care le acceptă serviciul, iar **Mai multe setări** arată doar opțiunile care modelează un bucket nou.
- **Google Drive și OneDrive.** Datele curățate se șterg definitiv, în loc să ajungă în coș, unde ar conta în continuare la cotă. Un drive partajat Google se pune la **Mai multe setări**.
- **Serverele SMB care nu își listează partajările** primesc numele uneia la **Partajare**, iar testul deschide direct acea partajare.
- **FTP** cere **Criptare**: **Fără**, **TLS explicit (FTPES)** sau **TLS implicit (FTPS, portul 990)**.
- **OneDrive și Zoho WorkDrive.** Când o autentificare deschide mai multe unități sau spații de lucru, fereastra întreabă în care ajung copiile de rezervă.
- **Un rest-server cu certificat autosemnat.** Pune certificatul ca fișier PEM în folderul `certs` din folderul de date al BombVault (`/config/certs` în container, `appdata/bombvault/certs` pe Unraid). Copiile de rezervă, copiile off-site și testul protecției la ștergere au încredere în el fără repornire.

Serviciile S3 rulează prin backendul S3 propriu al restic, ceea ce permite aplicarea unei clase de stocare și a object lock. Un rest-server este atins direct de restic. Orice alt serviciu rulează prin rclone-ul livrat cu BombVault, iar secțiunea lui din configurația rclone apare în cardul **rclone.conf** de pe pagina destinației, cu secretele ascunse. Un export de setări poartă destinațiile, iar cu credențialele incluse poartă și autentificările lor.

Dacă salvezi o destinație care deține deja copii de containere, VM-uri sau foldere, de exemplu una pe care ai eliminat-o și o adaugi din nou, ea se adaugă acelor secțiuni ca o copie nebifată. Nu se copiază nimic acolo până nu o bifezi, dar **Restaurare** oferă imediat copiile vechi.

Folderul de pe acest server se numește **Local** peste tot: pe paginile destinațiilor, în flux, la un element și la Restaurare. Dacă pe acest server există mai multe foldere, fiecare poartă numele lui propriu. O destinație pe care o creează câmpul off-site al unei secțiuni se numește după locul ei, adică ultimul folder din adresa ei sau, dacă nu are unul, după serverul ei, și primește un număr când o altă destinație are deja acest nume. Orice destinație poate fi redenumită în câmpul **Nume** din cardul ei **Conexiune**, la fel un folder de bucket pe care îl împart mai multe secțiuni și copia pe care o descrie câmpul off-site al unei secțiuni, iar un nume pe care i-l dai rămâne. Când două destinații cu același nume stau una lângă alta, fiecare poartă și serverul sau folderul ei.

### Pagina unei destinații {#location-page}

Fiecare destinație are o pagină proprie. Ea arată doar cardurile care se aplică acelei destinații, în această ordine:

![Pagina unei destinații: Dosare proprii, Conexiune și Credențiale.](assets/screenshots/storage/location-page-dark.png#only-dark)
![Pagina unei destinații: Dosare proprii, Conexiune și Credențiale.](assets/screenshots/storage/location-page-light.png#only-light)

- **Dosare proprii**: secțiunile care își scriu aici mai întâi copiile. Autentificarea, protecția și limitele acestei destinații li se aplică. **Folosește ca dosar propriu** mută aici dosarul propriu al mai multor secțiuni, iar **Schimbă** mută unul înapoi în altă parte. Vezi [Dosarul propriu al unei secțiuni](#own-folder).
- **Se salvează direct aici**: fiecare secțiune ale cărei elemente se salvează direct în acest loc, în locul dosarului lor propriu, și câte elemente fac asta. Un astfel de depozit direct își preia toate setările de la această destinație. Unul în care nu se mai salvează nimic spune asta, iar **Elimină** îl scoate din listă. Copiile de la locul lui rămân. Vezi [Depozite directe](#direct-repositories).
- **Conexiune**: **Nume**, comutatorul **Disponibil** (oprit înseamnă că BombVault nu mai scrie aici, iar elementele care se salvează aici eșuează cu un mesaj clar în loc să ajungă în altă parte), folderul, **Comprimare**, **Clasă de stocare**, **Conexiuni simultane** și **În afara sediului**. Pe destinația **Local** listează folderul fiecărei secțiuni la **Dosare proprii aici**. **Schimbă conexiunea** testează și salvează o adresă sau o autentificare nouă. Când alte locuri se autentifică cu aceeași autentificare salvată, fereastra spune asta, iar o schimbare de aici dă acestei destinații o autentificare proprie, în loc să rescrie pe cea partajată.
- **Credențiale**: ce autentificare salvată folosește (**Se autentifică cu**), ce alte destinații o mai folosesc (**Folosită și de**) și **Editează**, care deschide fereastra autentificării. Vezi [Autentificări](#sign-ins).
- **rclone.conf**: secțiunea acestei destinații din configurația rclone, pentru o destinație care rulează prin rclone.
- **Protecție împotriva ștergerii**, cu **Protecție împotriva ștergerii (append-only)**, **Acceptă fără protecție împotriva ștergerii**, **Testează acum protecția împotriva ștergerii** și **Cine curăță**, care spune dacă BombVault curăță copia și dosarele proprii de aici, după cardul **Cât timp se păstrează** al acestei destinații sau după valorile proprii ale unei secțiuni de la **Reguli pe secțiuni**, sau lasă asta în seama părții îndepărtate. Un folder local arată acest card doar când are o protecție pe care o poți opri. Vezi [Protecție împotriva ștergerii](#delete-protection-append-only).
- **Copiază**: **Când se copiază**, **Limită încărcare (KiB/s)** și **Limită descărcare (KiB/s)**. O destinație care ține doar dosare proprii și nicio copie arată limitele într-un card numit **Lățime de bandă**.
- **Cât timp se păstrează**: **Scurt**, **Echilibrat**, **Lung** sau **Valori proprii**, cu o axă a timpului a copiilor pe care le păstrează regula. Vezi [Cât timp se păstrează](#keep).
- **Deja sub această destinație**: secțiuni a căror adresă tastată manual se află într-un folder al acestei destinații. Preluarea uneia lasă depozitul unde este. Autentificarea, clasa de stocare și protecția la ștergere vin apoi de la destinație. BombVault verifică întâi dacă autentificarea destinației deschide depozitul și refuză să pună un depozit append-only sub o destinație care nu este append-only.
- **Reguli pe secțiuni**: depozitul, retenția sau comprimarea proprie a unei secțiuni la această destinație. **Schimbă regulile** le deschide, iar **Revino la destinație** readuce o secțiune. Autentificarea, lățimea de bandă și momentul copierilor vin mereu de la destinație.
- Un vas care arată cât de plină este destinația, cu marcajul unde se umple și câte săptămâni mai sunt la ritmul de creștere din ultimele săptămâni. O destinație care nu își raportează dimensiunea spune **Dimensiune necunoscută**.

**Mută** și **Elimină destinația** se află printre acțiunile paginii, descrise la [Mutare și eliminare](#move-remove).

O destinație la distanță care a avut copii de rezervă și dintr-odată nu mai are niciun depozit, cum ar fi un rest-server al cărui disc nu s-a montat, nu primește singură unul nou și gol. Pagina ei spune asta de îndată ce o copie de rezervă sau o copiere constată că depozitul lipsește. Montează din nou discul. Dacă ai golit intenționat depozitul vechi, **Creează aici un depozit nou** o ia de la capăt, pentru toate secțiunile destinației și depozitele ei deodată. Protecția este pentru o destinație care a rămas pe loc. O destinație pe care o adaugi, sau o secțiune pe care o bifezi la una, pornește curat: fără depozit la acea adresă, prima copiere creează unul, chiar și acolo unde o destinație eliminată mai devreme avea unul.

Două refuzuri numesc ce trebuie îndreptat, în rularea copierii și pe pagina destinației. **Acest bucket nu există pe server** apare imediat ce un server S3 spune asta, iar BombVault nu creează nimic acolo, așa că un nume de bucket scris greșit rămâne vizibil. Un server care nu răspunde deloc este alt caz: restic reîncearcă cam 15 minute pentru fiecare apel, în caz că revine. **Aici există deja un depozit restic, iar cheia acestei instanțe nu îl deschide** înseamnă că acele copii de rezervă aparțin altui `APP_KEY`.

Un server receptor pe care îl rulează altă instanță a grupului tău apare printre furnizori ca **Din grupul tău**. Vezi [Server receptor](#receiving-server).

BombVault verifică destinațiile la fiecare pornire. Îndreaptă singur ce nu cere nicio decizie de la tine, de exemplu o legătură spre o copie sau un depozit care nu mai există, o copie pe dosarul propriu al unei secțiuni sau un depozit direct a cărui copie a dispărut, și scrie o linie în jurnal pentru fiecare corecție. Ce nu are o corecție sigură este doar înregistrat în jurnal.

![Aceeași pagină pe un telefon.](assets/screenshots/storage/location-page-phone-dark.png#only-dark)
![Aceeași pagină pe un telefon.](assets/screenshots/storage/location-page-phone-light.png#only-light)

### Cât timp se păstrează {#keep}

Cât timp se păstrează copiile se stabilește pe fiecare destinație, în cardul ei **Cât timp se păstrează**. Regula acoperă copiile de la acea destinație și dosarele proprii care se află acolo, inclusiv destinația **Local**. O secțiune își poate păstra valorile ei la o destinație, la **Reguli pe secțiuni**, iar acestea primează față de cardul destinației pentru acea secțiune.

O regulă păstrează o copie pe zi, pe săptămână, pe lună și pe an, cât de departe în trecut stabilești, și curăță restul după fiecare copie de rezervă sau copiere:

| Presetare | Ultimele copii | Zile | Săptămâni | Luni | Ani |
|---|---|---|---|---|---|
| **Scurt** | 0 | 7 | 4 | 3 | 0 |
| **Echilibrat** | 0 | 7 | 4 | 6 | 2 |
| **Lung** | 0 | 14 | 8 | 12 | 3 |

**Valori proprii** îți lasă să stabilești singur cele cinci numere. Un 0 oprește acel număr, iar o regulă cu toate numerele pe 0 păstrează tot. Punctele din card arată care dintre 40 de copii eșantion le păstrează o regulă, cea mai nouă fiind în dreapta de tot. Eșantionul este același pentru fiecare destinație, pentru că imaginea se desenează înainte ca BombVault să fi citit copiile de acolo. Numără înapoi de azi, așa că numărul punctelor care rămân se schimbă odată cu data.

Un folder de pe această gazdă și un folder la distanță tastat manual au și ele un card propriu, care acoperă dosarele proprii din acel folder și niciun altul. Până îl salvezi, copiile dintr-un astfel de folder expiră după regula pe care BombVault o avea deja pentru el.

**Întreabă înainte de a păstra mai puțin**, pornit implicit, întreabă înainte să se salveze o schimbare care păstrează mai puține copii și spune câte dintre cele 40 de copii eșantion le elimină regula nouă. Se aplică fiecărei destinații. **Revino la destinație** de la **Reguli pe secțiuni** pune aceeași întrebare.

O destinație cu protecție la ștergere nu este niciodată curățată de pe această stație. Partea ei îndepărtată păstrează sau elimină copiile vechi după regulile ei. Cât timp un element are o anomalie critică deschisă despre date pierdute, retenția îi păstrează copiile vechi până o confirmi. Vezi [Anomalii](anomalies.md).

O copie de rezervă care conține mai puțin decât ar trebui nu le face niciodată să expire pe cele dinaintea ei. Asta este o copie care nu citește niciun fișier acolo unde cele anterioare aveau, ratează un folder selectat sau nu a putut citi unele fișiere. Rularea ei se încheie cu un avertisment care numește ce lipsește și spune că au fost păstrate copiile mai vechi, primești același mesaj pe canalele de notificare, iar nicio curățare, nici în dosarul propriu al elementului, nici la copiile lui, nu atinge copiile vechi ale acelui element până nu vine o copie completă. O primă copie de rezervă și un folder selectat care a fost mereu gol contează ca fiind complete.

Ca să vezi ce ar șterge cu adevărat următoarea rulare, deschide **Previzualizare** la **Setări, Destinații**. Ea listează **Ce ar șterge următoarea rulare** pentru dosarul propriu al secțiunii și pentru fiecare dintre copiile ei, pe depozit și pe element, cu punctele de restaurare numite. Nu ia niciun lock și nu schimbă nimic.

![Cât timp se păstrează cu Echilibrat ales: punctele arată care dintre cele 40 de copii eșantion rămân.](assets/screenshots/storage/keep-dark.png#only-dark)
![Cât timp se păstrează cu Echilibrat ales: punctele arată care dintre cele 40 de copii eșantion rămân.](assets/screenshots/storage/keep-light.png#only-light)

#### Exemplu: ce păstrează Echilibrat {#example-retention}

Un container este salvat în fiecare noapte la 03:00 din 1 octombrie 2024, așa că miercuri, 4 noiembrie 2026, are 765 de copii. Destinația lui este pe **Echilibrat**: 7 zile, 4 săptămâni, 6 luni și 2 ani.

Fiecare număr păstrează cea mai nouă copie a zilei, săptămânii, lunii sau anului lui, iar o copie pe care o aleg mai multe numere se păstrează o singură dată. Săptămânile merg de luni până duminică:

| Regulă | Copiile pe care le alege | Copii noi pe care le adaugă |
|---|---|---|
| 7 zile | 4 nov, 3 nov, 2 nov, 1 nov, 31 oct, 30 oct, 29 oct | toate 7 |
| 4 săptămâni | 4 nov, 1 nov, 25 oct, 18 oct | 25 oct, 18 oct |
| 6 luni | 4 nov, 31 oct, 30 sep, 31 aug, 31 iul, 30 iun | 30 sep, 31 aug, 31 iul, 30 iun |
| 2 ani | 4 nov, 31 dec 2025 | 31 dec 2025 |

Rămân 14 copii din 765. Cea mai veche este din 31 decembrie 2025, deci azi te poți întoarce puțin peste zece luni. Datorită deduplicării ocupă puțin mai mult decât o copie completă plus ce s-a schimbat între timp.

**Ani** merită o a doua privire. Numără anii calendaristici în care există o copie, începând cu cel curent, iar cea mai nouă copie din acest an este cea de azi. Așadar primul an nu adaugă nimic la celelalte numere, iar al doilea adaugă ultima copie din anul precedent. De aceea acoperirea unei reguli variază pe parcursul anului. Aceleași copii cu fiecare presetare:

| Presetare | Copii păstrate azi | Cea mai veche copie azi | Acoperire pe parcursul anului |
|---|---|---|---|
| **Scurt** | 10 | 30 septembrie 2026 | 1 până la 2 luni |
| **Echilibrat** | 14 | 31 decembrie 2025 | circa 4 luni la începutul primăverii, un an întreg pe 31 decembrie |
| **Lung** | 30 | 31 decembrie 2024 | 1 an pe 1 ianuarie, 2 ani pe 31 decembrie |

Trecerea de la **Echilibrat** la **Scurt** întreabă mai întâi, din cauza **Întreabă înainte de a păstra mai puțin**. Întrebarea spune câte dintre cele 40 de copii din imagine le elimină regula nouă. Numărul vine din eșantion, așa că se schimbă odată cu data și nu ține cont de copiile tale. Dintre copiile din acest exemplu, ar dispărea 31 august, 31 iulie, 30 iunie și 31 decembrie 2025. **Salvează oricum** aplică regula, iar următoarea curățare le șterge definitiv de la acea destinație.

Ca să ții containerele mai mult decât restul la această destinație, apasă **Schimbă regulile** la **Reguli pe secțiuni**, alege **Containere** și setează **Valori proprii** în **Reguli pentru Containere**, de exemplu 14 zile, 8 săptămâni, 12 luni și 2 ani. Celelalte secțiuni urmează în continuare cardul destinației, iar **Revino la destinație** readuce Containerele.

### Autentificări {#sign-ins}

O autentificare este cheia sau parola pe care BombVault o folosește pentru un serviciu S3 sau un rest-server. Autentificările sunt o singură listă, cardul **Autentificări** de la **Setări, Destinații**, iar fiecare rând spune ce destinații o folosesc (**Folosită de**).

- **Salvată când adaugi o destinație.** Adaugă destinație salvează cheile pe care le tastezi ca autentificare. Următoarea destinație la același serviciu o poate alege la **Autentificare salvată** în loc de **Chei noi**, iar o schimbare ulterioară a ei se aplică fiecărei destinații care o folosește.
- **Cea implicită.** O stea marchează autentificarea implicită. Se folosește oriunde nu este aleasă nicio autentificare, de exemplu la o adresă tastată manual sau la o recuperare pe o instalare nouă. O sursă de preluare nu o împrumută. **Fă-o implicită** dă steaua alteia.
- **Editează și Testează.** **Editează**, aici sau în cardul **Credențiale** al unei destinații, deschide o singură fereastră pentru autentificare. **Testează** se autentifică la fiecare destinație care o folosește, cu ce ai tastat, înainte să se salveze ceva. **Salvează** o stochează.
- **Eliminarea.** O autentificare poate fi eliminată doar când nicio destinație, nicio sursă de preluare și niciun depozit primit nu o folosește și nicio copie veche nu are nevoie de ea, iar cea implicită pleacă doar ca ultima. Copiile vechi au nevoie de ea când un dosar propriu s-a mutat dintr-un loc care se autentifică cu ea și le-a lăsat acolo. BombVault spune atunci care este acel loc.
- **Schimbarea conexiunii unei destinații** nu rescrie niciodată o autentificare pe care o împart și alte locuri. Destinația primește în loc una proprie, iar fereastra spune dinainte că autentificarea ei este partajată. Ca să schimbi autentificarea partajată peste tot, editeaz-o în cardul **Autentificări**.

![Cardul Autentificări: fiecare autentificare salvată cu ce o folosește și steaua pe cea implicită.](assets/screenshots/storage/signins-dark.png#only-dark)
![Cardul Autentificări: fiecare autentificare salvată cu ce o folosește și steaua pe cea implicită.](assets/screenshots/storage/signins-light.png#only-light)

Un folder de pe acest server nu are nevoie de autentificare. SMB, WebDAV, SFTP, Storage Box și unitățile cloud își țin autentificarea în configurația rclone, pe care o arată cardul **rclone.conf** al destinației, deci nu apar în această listă.

### SMB, WebDAV și alte remote-uri rclone {#smb-webdav}

O partajare Windows sau Samba și un server WebDAV (Nextcloud, ownCloud, SharePoint sau oricare altul) sunt plăci de furnizor în **Adaugă destinație**, la **NAS sau server**. Completează gazda și partajarea (SMB) sau adresa și tipul serverului (WebDAV), utilizatorul și parola, iar BombVault scrie secțiunea rclone pentru tine. rclone ofuscă parola înainte să fie stocată.

Pentru un serviciu pe care nu îl acoperă nicio placă, placa **Rclone** de la **Configurație rclone proprie** primește remote-uri lipite dintr-un rclone.conf. Ele se adaugă la remote-urile pe care BombVault le are deja și nu le înlocuiesc niciodată, iar sunt stocate criptat. Ca să salvezi o secțiune într-unul dintre ele, introdu `rclone:<remote>:<folder>` ca depozit off-site al acelei secțiuni, de exemplu `rclone:nas:backups/bombvault`. Partajarea sau bucket-ul este primul segment al căii, nu face parte din nume.

Aceasta este calea mai bună decât montarea partajării pe Unraid: restic nu recomandă păstrarea unui depozit pe o partajare CIFS montată, iar aici nu se montează nimic. NFS nu are placă pentru că nici restic, nici rclone nu au un backend NFS. Pentru NFS, montează exportul pe gazdă și folosește-l ca folder local.

### Mutare și eliminare {#move-remove}

- **Mută** duce copia unei secțiuni la altă destinație, cu tot istoricul ei. Totul rămâne în locul vechi. BombVault nu șterge nimic acolo. Mutarea rulează în fundal și poate dura mult: aproximativ 9 minute la 100 GB între discuri locale, ore pe o legătură lentă. Prezentarea o arată cât timp rulează.
- **Elimină destinația** oferă două căi. **Elimină destinația, păstrează copiile** nu mai scrie acolo și lasă copiile pe loc. Le poți citi mai târziu la **Restaurare, Dintr-un folder vechi de copii**. **Elimină destinația și șterge copiile de acolo** citește întâi ce este stocat acolo și îl listează: ce copii dispar, care dintre ele nu mai sunt nicăieri altundeva și ce depozite rămân pentru că protecția la ștergere refuză, elemente încă se salvează direct în ele, altă destinație sau secțiune își păstrează copiile acolo ori nu au putut fi citite. Elementele care și-ar pierde unica copie sunt listate separat și sunt păstrate dacă nu pornești **Șterge și acestea**. Confirmi tastând numele destinației, iar ștergerea rulează în fundal.
- Aceasta funcționează pentru orice fel de destinație: una folosită de mai multe secțiuni, o destinație configurată pentru o singură secțiune, un depozit creat de tine și folderul în care o secțiune își scrie propriile copii. Elementele care se salvau într-un depozit eliminat urmează din nou secțiunea lor, la fel ca o valoare implicită a secțiunii care indica acolo. Folderul unei secțiuni nu poate pur și simplu să dispară, fiindcă fiecare secțiune are nevoie de unul. Fereastra întreabă de aceea în ce folder se mută secțiunile, iar fiecare secțiune își păstrează dedesubt numele folderului ei.
- Ștergerea copiilor de la o destinație merge și fără parolă de conectare. Când este setată o parolă, cere o autentificare chiar pe instanță.

### Exemplu: o copie off-site pe Backblaze B2 {#example-s3}

Containerele și Auto-backup ar trebui să trăiască și la Backblaze B2, ca un server mort să nu le ia cu el. Aceiași pași se potrivesc la Wasabi, Hetzner Object Storage și la majoritatea celorlalte servicii S3.

1. **În B2,** creează un bucket privat, de exemplu `tower-backups`, într-o regiune apropiată de tine. La **Application Keys**, creează o cheie cu acces de citire și scriere, limitată la acel bucket, și notează ID-ul cheii și cheia aplicației. B2 arată cheia aplicației o singură dată. Setează ciclul de viață al bucket-ului să păstreze doar ultima versiune, altfel B2 păstrează datele curățate ca versiuni ascunse și ți le facturează.
2. **Furnizor.** În BombVault, deschide **Destinații**, apasă **Adaugă destinație** și alege **Backblaze B2** la **Serviciu S3**.
3. **Autentificare.** Introdu ID-ul cheii la **ID cheie de acces** și cheia aplicației la **Cheie de acces secretă**. Introdu endpoint-ul bucket-ului la **Punct de conectare**, de exemplu `s3.eu-central-003.backblazeb2.com`, iar BombVault citește regiunea din el. Tastează `tower-backups` la **Bucket**, pentru că această cheie deschide un singur bucket.
4. **Testează conexiunea.** Ar trebui să spună **Conexiune stabilită** și să listeze folderele bucket-ului. Apasă **Folder nou** și numește-l `bombvault`. BombVault pune în el câte un folder pe secțiune, de exemplu `bombvault/containers`.
5. **Protecție.** Fereastra explică faptul că B2 poate păstra versiuni vechi cu object lock și că BombVault nu poate verifica asta. Lasă **Protecție împotriva ștergerii (append-only)** oprită, în afară de cazul în care bucket-ul chiar refuză ștergerile, ca BombVault să poată curăța după regulile de păstrare ale destinației.
6. **Pentru ce secțiuni.** Bifează **Containere** și **Auto-backup**. Auto-backup poartă setările tale, ca o instalare nouă să le poată readuce. Fereastra spune câte elemente și, unde se știe, cât istoric aduce prima copiere.
7. Apasă **Salvează destinația**.

Prima copiere rulează după următoarea copie de rezervă. Ca s-o pornești acum, deschide **Setări, Destinații** și apasă **Replicează acum** în rândul secțiunii din **Copii externe**. Antetul de stare spune **Copierea off-site rulează**. Prima copiere aduce tot istoricul, așa că 200 GB pe un uplink de 100 Mbit/s durează aproximativ patru ore și jumătate. Copierile următoare trimit doar ce e nou.

Ca să-ți lași linia liberă, deschide pagina destinației și setează **Limită încărcare (KiB/s)** în cardul **Copiază**, de exemplu `5120`, care se citește ca aproximativ 5,0 MB/s (41,9 Mbit/s). Verifică și **Cât timp se păstrează** pe aceeași pagină, cât ești acolo.

Ca să vezi că a mers, uită-te la **Stare protecție** din prezentare, unde **Extern** devine **copie la zi**, și la fila **Copii de rezervă** a unui container, unde copiile lui numesc acum `tower-backups` ca al doilea loc. Apoi apasă din nou **Descarcă kitul de recuperare** la **Setări, Restaurare**, pentru că kitul listează noua destinație și autentificarea ei.

**Cloudflare R2 funcționează la fel,** cu un token de obiect în loc de o cheie. Lipește adresa S3 pe care o arată tabloul de bord R2, cu tot cu bucket, iar BombVault completează serverul și bucket-ul.

### Exemplu: un rest-server pe care îl rulezi tu {#example-rest-server}

Un rest-server în mod append-only refuză ștergerile, așa că o copie de acolo supraviețuiește chiar dacă cineva îți preia serverul. E de ajuns o a doua stație, la un prieten sau în garaj. Un BombVault împerecheat poate configura asta pentru tine, vezi [Server receptor](#receiving-server). Ca să-l rulezi manual pe orice gazdă Docker:

```sh
docker run -d --name rest-server --restart unless-stopped \
  -p 8000:8000 \
  -v /mnt/user/backups/rest-server:/data \
  -e OPTIONS="--append-only --private-repos" \
  restic/rest-server
docker exec -it rest-server create_user bombvault
```

Ultima linie cere o parolă și scrie utilizatorul în fișierul `.htpasswd` din folderul de date.

1. În BombVault, apasă **Adaugă destinație** și alege **rest-server** la **NAS sau server**.
2. Introdu adresa cu utilizatorul ca folder, de exemplu `http://192.168.2.20:8000/bombvault`, apoi **Nume de utilizator** și **Parolă**. Apasă **Testează conexiunea**.
3. La **Protecție**, pornește **Protecție împotriva ștergerii (append-only)**. BombVault nu mai curăță atunci acolo și lasă asta în seama părții îndepărtate.
4. Bifează secțiunile și apasă **Salvează destinația**.
5. După ce prima copie e acolo, deschide pagina destinației și apasă **Testează acum protecția împotriva ștergerii**. BombVault încearcă să șteargă un obiect de test. Cardul ar trebui să spună **Ultima dată: ștergere refuzată**, iar **Protecție împotriva ștergerii** din prezentare devine **verificată**. Dacă spune **Ultima dată: ștergere nerefuzată**, lipsește `--append-only`.

Pentru că nimic nu curăță această copie de aici, setează **Buget de creștere (GB, 0 = oprit)** în fereastra secțiunii, deschisă cu **Editează** în rândul ei din **Copii externe**, ca să afli înainte să se umple discul. Pentru un certificat autosemnat, vezi nota despre `certs` de mai sus.

## Dosarul propriu al unei secțiuni {#own-folder}

Fiecare secțiune are un dosar propriu: depozitul în care i se scriu mai întâi copiile. Copiile spre alte destinații pleacă de acolo. O instalare nouă ține dosarele proprii pe acest server, sub `/mnt/user/bombvault`, iar împreună ele formează destinația **Local**.

Un dosar propriu poate sta și într-o destinație pe care o ai deja, de exemplu un NAS, un rest-server sau un bucket S3. Secțiunea se salvează atunci direct acolo, fără o copie locală între ele, iar dosarul propriu urmează autentificarea, protecția la ștergere și limitele de lățime de bandă ale acelei destinații. BombVault îl curăță după fiecare copie conform cardului **Cât timp se păstrează** al destinației, iar valorile proprii ale unei secțiuni de la **Reguli pe secțiuni** au prioritate față de card. Destinația listează secțiunea în cardul ei **Dosare proprii**, iar **Destinații pe secțiuni** arată destinația ca fiind casa secțiunii.

### Mutarea unui dosar propriu {#move-own-folder}

1. Deschide destinația care ar trebui să țină dosarul propriu și alege **Folosește ca dosar propriu**, sau **Adaugă secțiuni** în cardul ei **Dosare proprii**. Ca să muți mai multe secțiuni odată, alege **Mută dosarele proprii** lângă **Dosare proprii aici**, în cardul **Conexiune** al destinației Local.
2. Alege secțiunile la **Secțiuni** și locul la **Mută în**. **Alt loc** primește un folder de pe această gazdă sau adresa unui depozit la distanță tastată manual, cu o autentificare dintre cele salvate.
3. Citește **Ce se întâmplă**. Pentru fiecare secțiune spune unde merg copiile noi, dacă o copie care se află deja la noul loc devine dosarul propriu și ce copii se copiază mai întâi.
4. **Copiază copiile de rezervă deja făcute** este pornit implicit. Dacă îl oprești, copiile vechi rămân unde sunt, dar elementul nu le mai listează. **Dintr-un folder vechi de copii** de pe pagina Restaurare le citește în continuare cu adresa folderului și, pentru un folder la distanță, cu autentificarea lui.
5. Alege **Comută**. Când cardul **Cât timp se păstrează** al noului loc păstrează mai puțin decât regula de azi a secțiunii, BombVault întreabă mai întâi **Păstrezi mai puțin?** Acolo, **Păstrează valorile actuale pentru** acea secțiune **la Reguli pe secțiuni** preia valorile de azi, **Salvează oricum** merge mai departe, iar **Înapoi** lasă totul cum era. Când copiile trebuie copiate întâi, BombVault le copiază și apoi comută dosarul propriu. Când sunt deja acolo, le verifică înainte de comutare.

După mutare, copiile noi merg în dosarul propriu mutat, și pentru o secțiune care avea **Local** oprit și se salva direct într-o copie. O copie care stătea deja la noul loc devine dosarul propriu, iar copierea acolo se încheie. Acolo unde acea copie păstra unele copii mai mult decât regulile de păstrare ale secțiunii, secțiunea păstrează tot ce păstrează oricare dintre reguli, așa că nu se pierde niciuna.

Când mutarea nu poate continua, fereastra spune de ce într-un chenar sub **Ce se întâmplă**. Când doar copierea copiilor vechi stă în cale, oprirea **Copiază copiile de rezervă deja făcute** lasă dosarul propriu să se mute fără ele.

O secțiune are mereu nevoie de un dosar propriu. Eliminarea destinației care îl ține cere folderul în care se mută secțiunea, vezi [Mutare și eliminare](#move-remove). Un dosar propriu pe lângă care trec copiile noi, pentru că secțiunea se salvează direct într-o copie, este marcat **(oprit)** acolo unde o destinație listează cine o folosește.

![Fereastra care mută un dosar propriu, cu Ce se întâmplă completat pentru o secțiune.](assets/screenshots/storage/move-own-folder-dark.png#only-dark)
![Fereastra care mută un dosar propriu, cu Ce se întâmplă completat pentru o secțiune.](assets/screenshots/storage/move-own-folder-light.png#only-light)

#### Exemplu: Containerele se mută pe un NAS {#example-own-folder}

Pool-ul cache se umple, iar tu ai o destinație numită `nas`, o partajare SMB pe o a doua stație. Secțiunea Containere are 61 de copii în dosarul ei propriu de pe acest server și copiază la Backblaze B2.

1. Deschide **Destinații**, apoi `nas`, și apasă **Folosește ca dosar propriu**.
2. Bifează **Containere** la **Secțiuni**.
3. Citește **Ce se întâmplă**. Spune că copiile noi ale Containerelor merg în folderul de pe `nas`, că cele 61 de copii din Local se copiază mai întâi acolo, că copierile spre Backblaze B2 continuă și că protecția, limitele de lățime de bandă și alarma de creștere vin de la `nas`.
4. Lasă pornit **Copiază copiile de rezervă deja făcute** și apasă **Comută**.

BombVault copiază copiile în fundal și apoi comută dosarul propriu. Pagina `nas` listează Containerele la **Dosare proprii**, un element care urmează secțiunea lui numește `nas` ca locuință, iar următoarea copie ajunge acolo, cu copierea spre B2 pornind de pe `nas`. De acum înainte, cardul **Cât timp se păstrează** al `nas` curăță dosarul. Vechiul folder de pe acest server rămâne unde este, iar BombVault nu șterge nimic acolo.

### Tastarea manuală a unei adrese la distanță {#remote-primary-repositories}

Cardul **Conexiune** al destinației Local primește în continuare o adresă la distanță pentru dosarul propriu al unei secțiuni: **Schimbă**, lângă secțiune, deschide o fereastră cu un comutator **Local / La distanță**. **La distanță** primește o adresă restic precum `s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo` sau `rclone:remote:bucket/path`, iar **Setări de securitate pentru repozitoriul primar la distanță** adaugă un test de conexiune, limite de încărcare și descărcare, protecție append-only cu același test de manipulare pe care îl primește o destinație off-site și o alarmă pentru bugetul de creștere.

O astfel de adresă se autentifică cu autentificarea implicită. Când este exact folderul pe care o destinație pe care o ai îl ține pentru secțiune, urmează singură acea destinație, inclusiv cardul destinației și comutatorul ei **Disponibil**, în afară de cazul în care se autentifică altfel sau este append-only acolo unde destinația nu este. O regulă care a curățat folderul până atunci rămâne ca regula proprie a secțiunii la **Reguli pe secțiuni**. Când se află într-un alt folder al acelei destinații, pagina acelei destinații listează secțiunea la **Deja sub această destinație**, iar preluarea ei îi dă autentificarea, clasa de stocare și protecția la ștergere ale destinației. Mutarea dosarului propriu într-o destinație cu **Folosește ca dosar propriu** este calea mai simplă, pentru că fereastra verifică copiile deja făcute înainte să se schimbe ceva.

## Amplasare per element {#placement}

Cardul **Programare și destinație** din fila Setări a unui element are un rând de butoane: **Local** și câte un buton pentru fiecare destinație off-site a secțiunii. Butoanele aprinse primesc copiile elementului.

- Cu **Local** aprins, elementul este scris în depozitul arătat la **Stocat pe** și copiat în fiecare altă destinație aprinsă. Întunecă o destinație și nu mai primește nimic nou de la acest element. Doar Local nu copiază nicăieri, ceea ce se potrivește datelor care au deja o a doua copie, de exemplu o partajare aflată pe un NAS.
- Cu **Local** întunecat, elementul este scris direct în depozitul direct al primei destinații aprinse și copiat de acolo în celelalte aprinse. Prima dată, o fereastră creează acel depozit direct.
- Un buton rămâne aprins, pentru că o copie are nevoie de un loc unde să meargă. Ca să lași un element în afara copiilor, folosește **Fără copie de rezervă**.

BombVault nu mută niciodată copii între depozite. Un element care urmează secțiunea lui o urmează și când valoarea implicită a secțiunii se schimbă: următoarea lui copie ajunge în locul nou, iar copiile pe care le are rămân unde sunt. Un loc ales chiar pe element rămâne fixat din prima lui copie. Cardul **Depozit** din fila Setări îl numește. Copiile se pot schimba oricând. O destinație care nu mai primește un element își păstrează copiile pe care le are și le taie la retenția ei la următoarea rulare off-site a secțiunii. Din destinațiile append-only nu se poate șterge.

Locul pe care un element l-a părăsit își păstrează copiile. În fila **Copii de rezervă** a elementului și la **Restaurare** apare ca **Doar copii de rezervă mai vechi**, o restaurare poate citi în continuare de acolo, iar o curățare de acolo lasă în pace aceste copii.

**Urmează setarea implicită** readuce un element cu alegere proprie la secțiunea lui. Întreabă întâi, spune unde merg atunci copiile noi și numește ce nu mai primește fiecare destinație și ce ar încărca următoarea rulare.

Sub rând, cardul spune unde merge elementul și ce există efectiv: câte locuri îl păstrează, când a fost văzută ultima oară fiecare destinație și dacă 3-2-1 este îndeplinit. Un loc este serverul cu datele originale, fiecare destinație off-site și fiecare depozit marcat **În afara sediului**. BombVault verifică copiile și locurile. Nu verifică partea „două suporturi” din 3-2-1.

### Valori implicite pe secțiuni {#placement-defaults}

Cardul **Destinații pe secțiuni** de pe pagina Destinații desenează valoarea implicită a fiecărei secțiuni ca un flux 3-2-1. Fiecare linie de la o secțiune la o destinație este un comutator, **Local** inclus, pentru fiecare secțiune, deci și pentru unitatea flash, seturile de date ZFS și Auto-backup. Un clic pe o linie sau pe un punct o comută imediat, iar liniile punctate sunt oprite. Doar oprirea lui **Local** pentru o secțiune întreabă mai întâi, în aceeași fereastră ca la cardul valorii implicite, pentru că secțiunea se salvează atunci direct într-o copie. **Anulează** lasă totul cum era. **Grafic / Comutatoare** arată același lucru ca rânduri de butoane. Un comutator hotărăște unde merg copiile noi. Elementele fără alegere proprie și folderele de proiect ale stack-urilor Compose urmează imediat, deci următoarea lor copie merge la locurile noi. Copiile deja stocate rămân unde sunt și pot fi restaurate în continuare de acolo. Unitatea flash, seturile de date ZFS și Auto-backup se salvează în folderul lor local și se copiază off-site din acea copie. Linia lor către **Local** poate fi oprită și ea. Copiile lor merg atunci direct la destinațiile lor off-site, iar un element ZFS cu replică și fără nicio destinație trimite doar replica.

Cu **Local** oprit și doar destinații off-site pornite, o secțiune face copia direct în ele, iar BombVault alege singur depozitul de acolo. Containerele, VM-urile și seturile de foldere merg în depozitul direct al primei destinații pornite, descris mai jos, și sunt copiate de acolo în celelalte. Unitatea flash, seturile de date ZFS și Auto-backup scriu fiecare copie în fiecare destinație pornită, iar fiecare destinație le păstrează după retenția ei proprie.

O secțiune cu toate liniile oprite spune **Fără copie**. Elementele ei nu primesc copii și contează ca neprotejate.

Elementele care și-au ales propriile destinații pe pagina lor le păstrează. Fluxul spune câte sunt într-o secțiune, de exemplu **2 intrări diferă**, și duce la ele: la fila Setări a elementului când este unul, la o listă cu locul unde se salvează fiecare când sunt mai multe. **Urmează setarea implicită** de acolo readuce un element.

O destinație off-site nouă primește fiecare element care nu este setat pe Local. Fereastra care o adaugă spune câte elemente și, unde se știe, cât istoric înseamnă asta.

### Depozite directe {#direct-repositories}

Un depozit direct este locul unde elementele fac copia direct pe o destinație off-site. BombVault îl pune lângă folderul destinației, cu `-direct` adăugat la nume sau, când destinația stă la rădăcina bucket-ului, a remote-ului sau a unui utilizator rest-server, într-un folder din ea numit după secțiune, de exemplu `containers-direct`. În fluxul de pe pagina Destinații este creat fără întrebare. Oprirea lui Local pentru un singur element, astfel încât o destinație fără depozit direct să devină locul lui, deschide o fereastră cu locația sugerată, de exemplu `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, și un test de conexiune care nu creează nimic. **Creează și folosește** creează depozitul și îndreaptă elementul spre el. Un depozit direct nu are setări proprii. Urmează comutatorul, autentificarea, clasa de stocare, limitele, setarea append-only și retenția destinației lui și se schimbă odată cu ele. Pagina destinației îl listează la **Se salvează direct aici**, cu fiecare secțiune care se salvează direct în el și câte elemente fac asta. Unul în care nu se mai salvează nimic spune asta acolo și poate fi eliminat, iar copiile de la locul lui rămân. Când o cheie nouă a destinației nu îl poate deschide, depozitul direct își păstrează cheia pe care o are, iar salvarea spune asta. Un element dintr-un depozit direct este copiat de acolo în celelalte destinații aprinse, niciodată în destinația căreia îi aparține depozitul. Copiile lui de rezervă poartă eticheta `bv:direct`, iar orice altă trecere de retenție le păstrează, deci un depozit direct care și-a pierdut legătura cu destinația lui nu îmbătrânește niciodată după regulile locale. B2 se atinge prin punctul său final S3, cu ID-ul cheii și cheia aplicației introduse ca chei S3. O cheie limitată la folderul propriu al destinației nu poate ajunge în folderul de lângă el, așa că limitează cheia la folderul de deasupra.

### În afara sediului {#off-the-premises}

O destinație sau un depozit poate fi marcat **În afara sediului**. Cele la distanță pornesc marcate. Oprește marcajul pentru un rest-server din aceeași clădire. Marcajul doar numără locurile și 3-2-1 pe carduri. Nu schimbă nicio copie.

O replică ZFS este o copie în afara sediului, dar nu o copie de rezervă cu istoric. Păstrează doar instantaneele pe care le stabilești, iar versiunile vechi și fișierele individuale vin tot din copiile de rezervă. Merge de pe un server ZFS direct pe celălalt, prin SSH către un server ZFS sau către punctul de primire al unei instanțe împerecheate. Grupul de împerechere poartă doar cererea. Vezi [Replică](zfs-datasets.md#replica).

### După o reconstruire {#after-a-rebuild}

Alegerile de copiere stau în setările proprii ale BombVault. După o reconstruire prin **Descoperă copii de rezervă** sau prin Asistentul de urgență fără setări restaurate, ele dispar, iar copierea a tot ar trimite din nou la B2 elementele pe care le lăsaseși deoparte. La fel se întâmplă când o secțiune care nu a copiat niciodată off-site din această bază de date găsește copii proprii la o destinație sau copii mai vechi în dosarul ei propriu, de exemplu după ce o destinație a fost adăugată peste copiile unei instalări anterioare cu același `APP_KEY`. Copiile off-site ale acelei secțiuni așteaptă atunci decizia ta.

**Necesită atenție** din prezentare și pagina fiecărei destinații spre care copiază secțiunea întreabă dacă acele copii sunt ale acestui server. **Preia-le ca fiind ale acestui server** confirmă valoarea implicită a secțiunii, iar copiile ei rulează din nou de la trecerea următoare. **Verifică mai întâi regulile** duce la rândul secțiunii din **Destinații pe secțiuni**, unde vezi unde merge fiecare element și poți seta din nou o regulă pierdută pe pagina elementului. Copierea așteaptă acolo până apeși **Preia-le ca fiind ale acestui server**, care arată mai întâi ce copiază rularea următoare și numele din copiile fără element, pe care le poți lăsa deoparte. Cea mai nouă copie a fiecărui element care așteaptă spune asta în rularea ei, cu o legătură spre decizie, iar o notificare pleacă atunci când notificările sunt pornite. Importul unui fișier de setări readuce regulile și valorile implicite, dar nu ia decizia în locul tău.

![Necesită atenție după o reconstruire: seturile de foldere au găsit copii de dinainte de această bază de date, iar copiile lor off-site așteaptă decizia.](assets/screenshots/offsite/copy-decision-dark.png#only-dark)
![Necesită atenție după o reconstruire: seturile de foldere au găsit copii de dinainte de această bază de date, iar copiile lor off-site așteaptă decizia.](assets/screenshots/offsite/copy-decision-light.png#only-light)

## Protecție împotriva ștergerii (append-only) {#delete-protection-append-only}

Pornește protecția împotriva ștergerii pentru o destinație off-site ca ransomware-ul sau o gazdă compromisă să nu poată șterge sau rescrie copiile tale. Partea îndepărtată (un `restic/rest-server` care rulează în modul `--append-only`) o **impune**. BombVault doar o **verifică** și nu arată niciodată verde pe baza unei simple afirmații de configurare. Un folder de pe acest server nu poate refuza ștergerea, pentru că cine controlează BombVault controlează și folderul, așa că destinațiile locale nu oferă comutatorul.

Comutatorul ține de destinație: o secțiune care copiază acolo îl preia de la destinație, așa că pornește-l de pe pagina destinației, la **Destinații**. Verificarea programată **Verifică protecția la ștergere** rulează doar cât timp o secțiune copiază către o destinație cu comutatorul pornit. Cealaltă parte trebuie pornită cu `--append-only`. Cu imaginea `restic/rest-server`, pe TrueNAS ca pe orice alt host Docker, asta intră în variabila `OPTIONS`.

Unele locuri nu pot refuza deloc o ștergere, de exemplu un disc în cloud precum OneDrive. Dacă păstrezi acolo copii în mod conștient, pornește **Acceptă fără protecție împotriva ștergerii** pe pagina destinației. Prezentarea generală arată atunci protecția lipsă a unei secțiuni care copiază doar în destinații acceptate ca risc acceptat, un inel gri în loc de un punct portocaliu, iar culoarea secțiunii urmează celelalte verificări. Nimic altceva nu se schimbă: copiile de acolo pot fi șterse în continuare, BombVault le curăță în continuare după regulile de păstrare ale destinației, iar comutatorul dispare imediat ce protecția împotriva ștergerii este pornită. O copie care se mută în alt loc pierde acceptarea. Pornirea protecției împotriva ștergerii o retrage și ea.

Cea mai rapidă cale spre o copie append-only este un BombVault împerecheat cu un [server receptor](#receiving-server): el rulează rest-server-ul pentru tine, în mod append-only.

!!! note "O ștergere reușită sub `/locks/` este de așteptat"
    Append-only nu înseamnă că nu se poate elimina niciodată nimic. restic trebuie să ia și să elibereze propriile lock-uri, așa că `/locks/` rămâne scriibil și ștergibil prin concepție. Instantaneele și datele din spatele lor, adică ținta ransomware-ului, nu pot fi eliminate. Dacă sondezi tu însuți partea îndepărtată, o ștergere reușită sub `/locks/` este comportament corect și nu o breșă în protecție.

!!! warning "Depozitele append-only nu sunt niciodată curățate de pe această stație"
    O destinație append-only nu curăță niciodată de aici copiile de rezervă vechi. Curățarea este treaba părții îndepărtate. Setează pentru ea o **alarmă pentru bugetul de creștere** ca să fii alertat înainte ca dimensiunea depozitului să scape de sub control.

## Test de manipulare {#tamper-test}

BombVault dovedește periodic garanția append-only încercând efectiv o ștergere în depozitul off-site, țintită la un obiect inexistent. **Testează acum protecția împotriva ștergerii** pe pagina unei destinații îl rulează la cerere, iar **Verifică protecția la ștergere** la **Setări, Verificare, Verificări automate** îl rulează după un program.

Oprirea **Verifică protecția la ștergere** nu ascunde nicio lipsă. O destinație cu protecție la ștergere care nu a fost niciodată testată apare atunci ca „nedovedită, verificarea ei este oprită”, iar una fără protecție la ștergere apare ca „niciuna” în orice caz.

O copie oprită nu este testată, iar destinația spune asta. Testul de conexiune merge în continuare, pentru că doar citește, deci o copie poate fi verificată înainte să fie pornită.

- **Refuzat** înseamnă protejat.
- **Acceptat** înseamnă neprotejat.
- Un rezultat **neconcludent** (server inaccesibil, eroare de autentificare) nu schimbă niciodată verdictul stocat.

O trecere reală de la protejat la neprotejat declanșează o singură alertă.

## Exerciții DR {#dr-drills}

BombVault oferă două niveluri de dovadă că copiile tale chiar se pot restaura, nu doar că există.

- **Exerciții de verificare a restaurării (local).** BombVault rulează periodic `restic check --read-data-subset` (limitat, niciodată o restaurare completă care umple discul). Cadența se află la **Setări, Verificare, Verificări automate**, iar cardul **Verifică copiile de rezervă** de pe aceeași placă arată rezultatul.
- **Restaurare de probă (off-site).** BombVault restaurează o țintă reală din depozitul off-site într-un sandbox izolat, o verifică fișier cu fișier și octet cu octet, apoi curăță. Asta dovedește că poți recupera din off-site, nu doar că depozitul răspunde.

Cardul **Stare protecție** din prezentare rezumă asta pe secțiuni în patru verificări: o copie actuală pe acest server, o copie off-site, protecție la ștergere acolo și o restaurare de probă de acolo. Un clic pe o secțiune arată detaliile, iar cardul devine verde doar pe fapte verificate. O destinație pe care ai acceptat-o fără protecție împotriva ștergerii contează acolo ca risc acceptat, nu ca avertisment.

## Împerecherea instanțelor {#pairing}

Mai multe BombVault-uri lucrează împreună ca un grup: își arată reciproc starea de protecție, preiau roluri unul pentru altul și își pot păstra reciproc cheia. o instanță se alătură grupului cu douăsprezece cuvinte.

![Pagina de împerechere cu cele trei carduri de pași.](assets/screenshots/offsite/pairing-dark.png#only-dark)
![Pagina de împerechere cu cele trei carduri de pași.](assets/screenshots/offsite/pairing-light.png#only-light)

Pe prima instanță deschide **Setări, Împerechere** și apasă **Generează frază**. Trei carduri de pași arată cum merge. Douăsprezece cuvinte apar într-o fereastră cu un buton **Copiază**. Pe fiecare altă instanță deschide același loc, apasă **Introdu frază** și lipește sau tastează cuvintele, sau apasă **Lipește** în acea fereastră. Un cuvânt care nu este în listă este numit cu poziția lui pe măsură ce tastezi, iar ultimul cuvânt poartă o sumă de control, așa că un cuvânt greșit sau schimbat este prins înainte să se împerecheze ceva. Generează fraza pe o singură instanță: două instanțe care generează fiecare una formează două grupuri separate. Dacă nu apare nimeni timp de un minut, pagina oferă două ieșiri: arată din nou cuvintele ca să le introduci dincolo, sau introdu cuvintele celeilalte instanțe și intră în grupul ei dintr-un singur pas. Împerecherea funcționează fără o parolă de conectare, dar setează una: fără ea oricine poate deschide interfața web poate citi cuvintele și, prin grup, poate cere roluri care dau mai departe parola restic a instanțelor din el. Cardul de împerechere spune asta până se setează o parolă. Cu o parolă, afișarea din nou a frazei o cere. **Părăsește grupul** scoate din nou o instanță. Cardul **Instanțe** de pe pagina de împerechere listează membrii grupului. Imaginea cu cine ce face pentru cine este pe pagina Instanțe.

Oricine cunoaște cuvintele poate intra în grup, așa că tratează-le ca pe o parolă.

**Cum se ajung membrii.** Fiecare instanță își află propria adresă din rețea de la browserul tău în clipa în care te autentifici, listată la **Adrese cunoscute** în cardul **Această instanță** de pe pagina de împerechere împreună cu cele găsite de instanță însăși. Corectează-o acolo dacă în față stă un reverse proxy sau un port neobișnuit. În aceeași rețea membrii anunță acea adresă prin multicast și vorbesc direct, iar acolo unde multicastul nu poate trece printr-o rețea de containere, cum este puntea implicită Docker, o instanță își caută în schimb propria subrețea pentru ceilalți cu un apel semnat la care poate răspunde doar un membru al grupului, așa că împerecherea se încheie tot în câteva secunde fără relay. Dacă nu apare nimic, **Nu o găsești?** primește o adresă introdusă manual, pentru altă subrețea sau un port neobișnuit. Instanțele din rețele diferite trec printr-un relay, ales într-un singur rând de patru, în cardul **Relay** de pe aceeași pagină. Fiecare formă vine cu o imagine:

- **Relay-ul proiectului** (implicit): `parleyport.halleluja.design`, relay-ul pe care îl folosește și KnightLoader. Nu ai nimic de configurat.
- **Relay propriu**: containerul [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) din Unraid Community Apps (și la **Setări, Aplicații**). Introdu adresa lui pe fiecare instanță care ar trebui să-l folosească.
- **Instanță BombVault**: una dintre instanțele tale care este deja accesibilă din exterior, cu **Servește ca relay** pornit. Acea instanță răspunde atunci la `/relay/connect` pe propria adresă, în spatele reverse proxy-ului și certificatului pe care le are deja, și lasă să intre doar grupul tău. Introdu adresa ei pe fiecare instanță care ar trebui să o folosească.
- **Fără relay**: membrii se găsesc automat în aceeași rețea și nicăieri altundeva.

**Ce vede relay-ul.** Un buton cu același nume, lângă cele patru forme, arată asta pentru forma aleasă. Fiecare apel între membri este sigilat cu AES-256-GCM sub o cheie derivată din cele douăsprezece cuvinte, iar acea cheie nu părăsește niciodată instanțele tale. Relay-ul află un hash care grupează conexiunile, pentru ce instanță este un mesaj, cât de mare e și când trece. Un apel direct în rețeaua locală este sigilat la fel și semnat în plus, așa că nimic nu depinde de certificatul autosemnat pe care îl servește o instanță. Apelurile care dau ceva mai departe (răspunsuri la roluri, o parolă restic, autentificări ale receptorului, depuneri de chei) sunt semnate și cu o cheie a membrului care le trimite, fixată la prima vedere. Vezi [Model de securitate](configuration.md#security-model).

**Ce circulă prin grup.** Starea de protecție de pe pagina Instanțe, o cerere de a verifica acum o secțiune, cererile de roluri și răspunsurile lor, ofertele de stocare și ce are nevoie un receptor sau un preluator odată ce rolul i-a fost permis: locațiile depozitelor celeilalte instanțe și parola ei restic. Datele de copie nu circulă niciodată. Merg tot direct la backendurile restic. Nici APP_KEY: parola restic deschide depozitele acelei instanțe și nimic altceva, nu secretele ei stocate, sesiunile sau codurile de recuperare.

**Intrări de dinainte de împerechere.** Instanțele adăugate cu un token de flotă și depozitele primite și sursele de preluare configurate cu APP_KEY-ul celeilalte instanțe rămân după actualizare și sunt marcate **Împerechează din nou**. Continuă să funcționeze: la prima pornire, BombVault înlocuiește fiecare APP_KEY stocat cu parola restic derivată din el. Împerechează ambele instanțe, apoi editează intrarea și alege instanța ei. Până atunci astfel de intrări stau în cardul **Fără instanță** de pe pagina Instanțe.

Singurul loc care mai primește un APP_KEY introdus manual este [Dintr-un folder vechi de copii](#restore-from-another-bombvault-repo), pentru cazul în care cealaltă instanță a dispărut și nu poate răspunde într-un grup.

## Instanțe și roluri {#roles}

Pagina **Instanțe** (o placă de setări care poate fi fixată în bara laterală) arată grupul ca o rețea cu cine ce face pentru cine, un card pentru această instanță și un card pentru fiecare alt membru, telefoanele Android și serverele ZFS. **Împerechere**, jos în dreapta, duce la pagina de împerechere. Ce părți sunt pornite se stabilește la **Setări, Instanțe, Mai multe servere**: **Instanțe**, **Receptor** și **Preluare**.

![Pagina Instanțe: grupul ca o rețea și câte un card pentru fiecare instanță, cu plăcile de roluri, una dintre ele Inactivă.](assets/screenshots/offsite/instances-dark.png#only-dark)
![Pagina Instanțe: grupul ca o rețea și câte un card pentru fiecare instanță, cu plăcile de roluri, una dintre ele Inactivă.](assets/screenshots/offsite/instances-light.png#only-light)

Fiecare card arată starea de protecție a membrului și rolurile lui ca plăci. Un rol este ceva ce o instanță face pentru alta:

- **Receptor** păstrează copiile off-site ale acestui server într-un rest-server în mod append-only. Acest server trimite după fiecare copie și nu poate șterge nimic acolo, chiar dacă îl preia cineva.
- **Preluator** preia copiile acestui server după un program propriu. Acest server nu trebuie să fie treaz pentru asta. Instanța care preia are nevoie de **Preluare** pornită înainte să poată permite cererea.
- **Server ZFS** preia replicile ZFS ale acestui server. Fiecare element ZFS îl alege la **Replică** în setările lui.
- **Păstrător de cheie** păstrează o copie sigilată a kitului de recuperare al acestui server. Vezi [Păstrătorul de cheie](#key-keeper).

Pornirea unui rol trimite o cerere, și nimic nu începe până când cineva de pe cealaltă instanță apasă **Permite**. Cererea apare pe cardul acelei instanțe ca insignă, iar **Detalii** o listează cu **Permite** și **Refuză**. Un rol permis poate fi revocat acolo oricând. O linie punctată în rețea este un rol care încă așteaptă. O instanță cu BombVault 9.x nu poate răspunde la o cerere, deci un rol este activ acolo imediat ce îl pornești, iar o cerere de la o instanță 9.x așteaptă aici **Permite**.

**Detalii** pe un card deschide restul: depozitele primite și sursele de preluare ale acelui membru, ofertele de stocare, monitorizarea, **Elimină** și **Uită cheia**.

Un membru își poate oferi propria stocare off-site altui membru prin grup. Celălalt administrator vede oferta în **Detalii** ale membrului și o acceptă sau o refuză. Acceptarea creează o autentificare salvată și o destinație obișnuită, după aceleași verificări ca o destinație pe care o introduci tu. Destinația vine cu protecția la ștergere pornită, pentru că rețeta pe care o arată oferta este un rest-server append-only, așa că **Cât timp se păstrează** nu se aplică acolo. Fiecare ofertă are o autentificare și un folder proprii, așa că oferirea din nou a unei secțiuni, aceluiași membru sau altuia, lasă în pace copiile primului partener: adaugă noul rând în `.htpasswd` și lasă serverul să ruleze. Pe această cale circulă doar datele de conexiune, niciodată date de backup. **Verifică acum** cere unui membru să verifice depozitul unei secțiuni. Nimic de pe pagină nu poate porni un backup, o restaurare sau șterge ceva pe altă stație.

Instanțele oprite și telefoanele cu aplicația închisă își păstrează cardul ca **Inactivă**, în gri, cu momentul în care au fost văzute ultima dată, și stau estompate în rețea până când le elimini cu **Elimină**. Imediat după pornirea acestui server, un astfel de card arată **Se verifică** până când grupul revine. **Elimină** funcționează pe un astfel de card indiferent ce părți sunt pornite și încheie și cererile deschise ale membrului, locurile lui de recepție ZFS nefolosite și cererile lui de chei în așteptare. Cardul **Instanțe** de pe pagina de împerechere listează și el acești membri ca **Inactivă**.

## Receptor și preluator {#receiver}

Pe stația care **primește** copii append-only de la alt BombVault, depozitele primite îți oferă monitorizare independentă, doar în citire, pe hardware-ul care primește, ca un eșec silențios de la capătul îndepărtat să nu treacă neobservat. Pornește **Receptor** la **Setări, Instanțe**, apoi deschide **Detalii** ale instanței care trimite, pe pagina Instanțe, și adaugă depozitul primit (doar în citire, deschis cu parola restic a instanței care trimite, pe care o primește prin [grupul de împerechere](#pairing)). Primești:

- **Un inventar pe surse**, ca să vezi exact ce containere, VM-uri și seturi de foldere au ajuns. Copiile făcute de instanța care trimite înainte să aibă o [gazdă](#backup-host) proprie se socotesc la aceeași sursă.
- **Ultima primire** pe sursă, ca să știi cât de proaspătă este fiecare.
- **Un `restic check` independent** rulat pe hardware-ul care primește, ca integritatea să fie verificată acolo unde se află datele, nu doar la expeditor.
- **Un comutator de om mort:** o alertă când o sursă nu mai trimite într-o fereastră stabilită de tine. Se uită o dată pe zi, la 09:15. Primești o alertă și când depozitul nu poate fi citit deloc, indiferent de programul verificării, și când nu a sosit nimic la o fereastră întreagă după ce l-ai adăugat. Un depozit pe care îl adaugi înaintea primei copii a expeditorului este în regulă până atunci.
- **Alerte de integritate:** o alertă când o verificare de pe partea care primește eșuează. O verificare care nu a putut deschide depozitul nu spune nimic despre integritate, așa că nu lasă niciun verdict și încearcă din nou a doua zi la aceeași oră.

Un depozit de pe alt server are nevoie de autentificarea acelui server: salveaz-o ca [autentificare](#sign-ins) și alege-o la **Credențiale** în dialog.

Receptorul este strict doar în citire. Nu scrie niciodată în depozitul primit, deci nu poate strica niciodată garanția append-only pe care se bazează expeditorul.

Un preluator lucrează invers: cu **Preluare** pornită, o instanță citește copiile altui membru după un program propriu, într-un depozit propriu pentru fiecare sursă, așa că ele nu contează niciodată drept copiile acestei stații. Sursele lui stau și ele în detaliile acelui membru: locul depozitului, ce fel de copie ține și cât de des se preia. Parola restic vine prin grup, niciodată APP_KEY, iar partea îndepărtată nu are nimic în plus de configurat și nu trebuie să ruleze. Sursa este doar citită, iar fiecare parte își păstrează autentificarea proprie. O preluare rulează în ziua și la ora pe care le spune programul ei, iar o sursă nouă așteaptă și ea acea oră. **Preia acum** nu așteaptă. O preluare ratată pentru că stația era oprită rulează după ce stația revine. Una care a eșuat încearcă din nou a doua zi la aceeași oră, și la un program săptămânal, iar o preluare eșuată apare în jurnalul de activitate și trimite o singură alertă până când o preluare reușește din nou. O sursă `rclone:` este refuzată, pentru că rclone ar ajunge la ea cu remote-urile acestei instanțe.

O copie preluată doar crește. BombVault adaugă la ea și nici nu o curăță, nici nu o verifică, așa că ce a uitat sursa de mult rămâne aici până ștergi folderul, iar pagina Destinații nu o socotește. Se află în `pulled/<source id>` în dosarul propriu al secțiunii, iar când acel dosar se mută, copia rămâne unde este și preluările următoare continuă acolo, în loc să copieze totul din nou. Dacă o ștergi, următoarea preluare o ia de la capăt în dosarul propriu pe care îl are secțiunea atunci.

### Server receptor {#receiving-server}

Stația care primește poate rula și rest-server-ul spre care copiază ceilalți. Se configurează prima dată când permiți o cerere **Receptor** care îl cere: fereastra spune că serverul are nevoie de un server receptor și cere un folder pe o partajare, cu **Dosar nou** pentru a crea unul, și un port (8000, dacă nu-l folosește alt container). BombVault apoi:

1. refuză dacă există deja un container numit `rest-server` sau dacă alt container ține portul,
2. trage `restic/rest-server` și îl pornește prin socketul Docker în mod append-only, cu depozite private și un fișier de autentificare în folder,
3. scrie șablonul lui Unraid pe unitatea flash, ca containerul să rămână editabil în fila Docker, sau oferă șablonul ca descărcare când unitatea flash nu poate fi atinsă,
4. rulează testul de manipulare împotriva lui și arată dacă refuză ștergerile.

Instanțele din grupul tău găsesc apoi serverul printre furnizori la **Din grupul tău**, numit după stația care primește, iar rolul **Receptor** îl oferă ca alegerea sigură. Fiecare instanță primește o autentificare proprie prima dată când folosește serverul și scrie doar în propriul folder de acolo. Cu **Receptor** pornit la **Setări, Instanțe**, **Server receptor** pe cardul acestei instanțe redeschide serverul. Fereastra lui listează autentificările la **Parteneri cu autentificare**, iar **Revocă autentificarea** o scoate pe una. Ce a copiat deja acea instanță rămâne în folder. Configurarea creează și o **Autentificare din afara grupului**, a cărei parolă fereastra o arată o singură dată.

O instanță care ajunge la stația receptoare doar prin relay nu poate folosi serverul, pentru că relay-ul nu transportă copii. Adaugă mai întâi adresa stației receptoare pe pagina de împerechere. Când BombVault rulează pe o adresă IP proprie (pe br0, de exemplu), completează **Adresă pentru parteneri**, pentru că serverul ascultă pe adresa gazdei.

## Exemplu: împerecherea a două stații Unraid {#worked-example-two-unraid-boxes-end-to-end}

Aceasta este o configurare completă a pieselor de mai sus, pentru că se asamblează mai ușor după ce le-ai văzut asamblate o dată.

Două stații: **TOWER** rulează containerele și trimite copiile. **VAULT** le primește și impune append-only. Înlocuiește cu propriile nume, adrese și căi de partajare.

**1. Împerechează cele două stații.** Pe TOWER deschide **Setări, Împerechere**, apasă **Generează frază** și introdu cele douăsprezece cuvinte pe VAULT la **Introdu frază**. Setează mai întâi o parolă de conectare pe amândouă.

**2. Pe TOWER, cere ca VAULT să fie receptor.** Pe pagina Instanțe, pornește **Receptor** pe cardul VAULT, alege secțiunile și **serverul receptor al VAULT** ca loc și trimite cererea.

**3. Pe VAULT, permite.** Pagina Instanțe a VAULT arată cererea pe cardul TOWER. Apasă **Permite**. Prima dată, VAULT cere un folder pentru serverul lui receptor, de exemplu `user/appdata/rest-server`, pornește rest-server-ul în mod append-only și rulează testul de manipulare împotriva lui.

**4. Înapoi pe TOWER.** De atunci TOWER copiază spre VAULT după fiecare copie de rezervă, iar noua destinație apare pe pagina Destinații a TOWER cu protecția ei la ștergere.

Ce răspunde testul de manipulare:

| Rezultat | Ce s-a întâmplat |
| --- | --- |
| **protejat** | VAULT a refuzat ștergerea. Aceasta este singura stare de succes. |
| **neprotejat** | VAULT a acceptat o ștergere. `--append-only` lipsește sau a fost scos. |
| **neconcludent** | Niciuna. De obicei URL-ul nu este cel pe care îl folosește restic însuși sau credențialele s-au schimbat. Nu se înregistrează nimic și nu pornește nicio alertă. |

**5. Urmărește ce sosește, dacă vrei.** Pornește **Receptor** la **Setări, Instanțe** pe VAULT, deschide **Detalii** ale TOWER și adaugă depozitul primit: fă clic pe unul dintre locurile pe care le listează dialogul. VAULT primește parola restic a TOWER prin grup, iar pentru că locul se află pe propriul server receptor al VAULT, citește folderul direct. Nimeni nu tastează o cheie sau o autentificare.

**6. Fă-l reciproc.** Repetă pașii în sens invers: VAULT cere ca TOWER să-i fie receptor. Fiecare stație impune atunci append-only pentru cealaltă, și niciuna nu poate șterge copiile celeilalte.

Un rest-server pe care îl rulezi manual funcționează și el: adaugă-l cu **Adaugă destinație** ca rest-server, cu utilizatorul și parola din `.htpasswd`, și pornește protecția la ștergere. Păstrează `--append-only` în opțiunile lui. Fără el partea îndepărtată este din nou o partajare obișnuită.

!!! warning "O cale locală este o cale din interiorul containerului"
    Acolo unde BombVault cere un folder de pe acest server, introdu-l relativ la montarea gazdei, de exemplu `user/appdata/rest-server/bombvault-containers`, nu `/mnt/user/appdata/...`. BombVault rulează într-un container, unde `/mnt` al gazdei este montat în alt loc. O cale absolută de gazdă nu există în interior. Dacă lipești una, BombVault îți spune calea relativă de folosit în loc.

## Asistent de urgență {#emergency}

**Asistentul de urgență** readuce un server pierdut sau reinstalat. Pornește pe pagina **Restaurare** la **Urgență** și de la **Restaurează dintr-o copie de rezervă** pe pagina de bun venit a unei instalări noi. Rulează în patru pași: **Acces**, **Setări**, **Selecție** și **Restaurare**.

1. **Acces.** Arată BombVault cheia vechilor copii, în unul din trei moduri:
    - **Încarcă kitul de recuperare**: fișierul descărcat în timpul configurării. Conține cheia și fiecare destinație, iar BombVault completează restul din el. Un kit sigilat cu age cere identitatea pentru care a fost sigilat. Este folosită o singură dată și nu este stocată.
    - **Prin grup**: o instanță a grupului tău păstrează cheia ca depunere. Ai nevoie de cele douăsprezece cuvinte ale grupului, de codul tău de deblocare și de cineva la acea instanță. Vezi [Păstrătorul de cheie](#key-keeper).
    - **Introdu cheia**: `APP_KEY` al vechiului server și unde sunt copiile lui.

    BombVault încearcă apoi cheia pe fiecare destinație și spune pentru fiecare dacă este lizibilă, negăsită, blocată cu altă cheie, inaccesibilă sau încă fără autentificare. Încă nu s-a salvat nimic, iar cheia rămâne în memorie un timp limitat. **Preia cheia** face ca acest server să-și scrie de acum copiile cu acea cheie. **Anulează preluarea** revine la propria cheie până rulează prima copie. Un server care are deja copii proprii refuză preluarea și trimite la [Dintr-un folder vechi de copii](#restore-from-another-bombvault-repo). **Salvează credențialele** stochează autentificările pe care le poartă kitul pentru destinațiile la distanță și întreabă înainte să le înlocuiască pe cele pe care le are deja acest server.
2. **Setări.** **Readu setările** restaurează propriile setări ale BombVault din cea mai nouă copie Auto-backup: destinații, credențiale, programări și reguli. BombVault repornește ca să le aplice, prin socketul Docker, deci baza de date activă de setări nu este niciodată suprascrisă sub un handle deschis, iar asistentul continuă singur după aceea. Autentificarea acestui server rămâne: continui să te autentifici cu parola pe care o folosești acum. Secretele stocate de vechiul server care nu pot fi preluate sunt listate, ca să le poți introduce din nou. Restaurarea încheie toate sesiunile și elimină cheile de acces, așa că configurează-le din nou după aceea.
3. **Selecție.** BombVault caută prin copiile tale și listează containerele, VM-urile, seturile de foldere și seturile de date ZFS pe care le găsește. Cea mai nouă copie de rezervă este preselectată peste tot. Ce are încă acest server pornește oprit, pentru că o restaurare l-ar înlocui. Seturile de foldere nu mai știu de unde au venit după o reconstruire, așa că merg într-un folder ales de tine.
4. **Restaurare.** Tot ce este selectat revine dintr-o dată, lăsat oprit dacă vrei să pornești tu fiecare container și VM, ceea ce este cel mai sigur după o pierdere totală. După aceea descarcă un kit de recuperare nou: cel vechi aparține instalării anterioare.

![Asistentul de urgență la primul pas, cu cele trei căi de acces.](assets/screenshots/offsite/emergency-dark.png#only-dark)
![Asistentul de urgență la primul pas, cu cele trei căi de acces.](assets/screenshots/offsite/emergency-light.png#only-light)

### Exemplu: un server nou din kitul de recuperare {#example-emergency}

Serverul TOWER a murit odată cu discurile lui. Ținea 23 de containere și 2 VM-uri și le copiase, împreună cu Auto-backup, la Backblaze B2. Ai `bombvault-recovery-kit.md` din managerul tău de parole și o stație nouă cu Unraid. Vechiul ei `APP_KEY` a dispărut, și nu ai nevoie de el.

1. **Instalează BombVault** pe stația nouă și dă-i un `APP_KEY` nou, ca la orice instalare nouă. Configurează legătura VM la **Setări, Integrări, Host SSH** dacă vrei VM-urile înapoi, pentru că o restaurare de VM are nevoie de ea.
2. **Pornește asistentul.** La **Bun venit în BombVault**, alege **Restaurează dintr-o copie de rezervă**. Pe pagina Restaurare, alege **Pornește asistentul de urgență** la **Urgență**.
3. **Acces.** Alege **Încarcă kitul de recuperare** și trage fișierul. BombVault încearcă cheia lui pe fiecare destinație și spune, de exemplu, **Kit de recuperare citit. Cheia lui deschide 0 din 2 destinații.** Folderul local spune **negăsit**, pentru că discul a murit, iar Backblaze B2 spune **lipsește autentificarea**.
4. **Salvează credențialele.** Kitul poartă autentificarea fiecărei destinații. Apasă **Salvează credențialele**, iar destinațiile sunt încercate din nou, așa că Backblaze B2 spune acum **lizibilă**. Un kit scris înainte de 10.5 conține doar autentificarea implicită. Acolo unde o destinație spune încă **lipsesc credențialele**, introdu-i autentificarea la **Introdu credențialele lipsă** și apasă **Încearcă din nou destinațiile**.
5. **Preia cheia.** Acest server își scrie acum copiile cu vechea cheie, deci copiile noi se potrivesc cu cele vechi. Își păstrează propriul `APP_KEY` în șablon. **Anulează preluarea** merge până rulează prima copie de rezervă.
6. **Setări.** Pasul se deschide pe locul unde cheia a găsit copia Auto-backup, aici B2, cu adresa deja completată. Alege **Readu setările**, care citește cea mai nouă copie Auto-backup de acolo. BombVault repornește ca s-o aplice, iar asistentul continuă singur. Destinațiile, programările și regulile tale sunt înapoi, și continui să te autentifici cu parola acestui server. Introdu din nou secretele pe care le listează asistentul, de exemplu setările de notificare.
7. **Selecție.** **Unde sunt copiile de rezervă?** se deschide pe copiile off-site când nu a mai rămas nimic local. BombVault citește fiecare destinație, ca una moartă să nu le ascundă pe celelalte, listează cele 23 de containere și 2 VM-uri și îl restaurează pe fiecare din destinația care ține cel mai nou instantaneu al lui.
8. **Restaurare.** Pornește **Lasă oprit după aceea**, ceea ce este cel mai sigur după o pierdere totală, și începe. **Ce se întâmplă** spune ce revine. Linia de rezultat spune, de exemplu, **25 restaurate, 0 eșuate. Pornește-le din Copii de rezervă când ai nevoie.**
9. **Final.** Pornește mai întâi bazele de date, apoi aplicațiile care le folosesc. Deschide **Setări, Restaurare** și descarcă un kit de recuperare nou, pentru că cel vechi aparține instalării anterioare. Verifică **Necesită atenție** în prezentare și rulează **Copiază tot acum** când totul funcționează.

Dacă lipsește și Auto-backup, pasul se deschide pe **Sari peste, nu am Auto-backup** și spune asta. Destinațiile vin atunci doar din kit, îți configurezi din nou programările și regulile, iar copiile off-site așteaptă până când preiei copiile lor ca fiind ale acestui server, vezi [După o reconstruire](#after-a-rebuild). **Selecție** listează în continuare ce conțin copiile off-site. Seturile de foldere ajung într-un folder ales de tine. Containerele și VM-urile revin ca fișiere simple într-un folder, la **Doar fișierele pot reveni**, pentru că definițiile lor se află lângă depozitul local și în Auto-backup, iar o copie off-site nu conține niciuna dintre ele.

!!! tip "Migrare planificată versus dezastru"
    Asistentul de urgență restaurează propriile setări ale BombVault dintr-o copie. Pentru o mutare *planificată* pe o stație nouă poți în schimb să-ți muți configurația direct cu cardul **Exportă / importă setările** (un fișier JSON portabil). Vezi [Configurare](configuration.md#portable-settings-export-and-import).

### Dintr-un folder vechi de copii {#restore-from-another-bombvault-repo}

**Dintr-un folder vechi de copii**, la **Alte surse** pe pagina Restaurare, deschide depozitul unei instanțe BombVault *diferite* (o partajare montată sub `/mnt` sau o adresă la distanță) cu kitul de recuperare al acelei instanțe sau cu `APP_KEY` tastat, într-o sesiune unică, doar în citire. Răsfoiește containerele, VM-urile, seturile de foldere și seturile de date ZFS stocate acolo, alege o copie de rezervă și restaurează-o, iar obiectul restaurat devine un container, VM sau set de foldere local obișnuit. Nimic nu se scrie vreodată în celălalt depozit, iar setările tale de copii rămân neatinse (sesiunea trăiește în memorie și expiră singură). Mutarea unui container de pe serverul A pe serverul B nu înseamnă să-ți reîndrepți setările și să le reîntorci după aceea. Această fereastră este una unică: deschide o sesiune, restaurează ce alegi și uită cealaltă instanță. Dacă vrei în schimb o rânduială permanentă, în care această stație preia după un program copiile de rezervă ale altei instanțe în propriul depozit, acesta este rolul **Preluator** de pe pagina Instanțe.

### Ce a preluat acest server {#restore-what-was-fetched}

Fiecare sursă de la care a preluat acest server are propriul rând **Preluat de la** la **Alte surse** pe pagina Restaurare. O copie preluată este încuiată cu cheia proprie a acestui server, nu cu a sursei, așa că nu se cere nimic: cheia sursei nu o deschide. Seturile de foldere și seturile de date ZFS ajung într-un folder ales de tine. Containerele și VM-urile revin ca fișiere simple într-un folder, pentru că o preluare aduce instantaneele, nu și definițiile. Ca să le recreezi, folosește kitul de recuperare și Auto-backup-ul sursei în Asistentul de urgență.

Un container a cărui rețea nu există pe acest server, de exemplu o rețea Unraid `br0` pe o gazdă Docker simplă, arată sub rândul lui un selector de rețea. BombVault o creează în rețeaua pe care o alegi, împreună cu celelalte rețele ale lui. IP-ul fix și adresa MAC aparțineau vechii rețele și sunt eliminate, așa că rețeaua nouă le atribuie.

## Păstrător de cheie {#key-keeper}

Un păstrător de cheie este o instanță a grupului tău care ține o copie sigilată a kitului de recuperare al acestui server. Nu poate deschide singur copia. Dacă acest server se pierde vreodată, o instalare nouă care se alătură grupului tău cere copia de la păstrător și o deschide cu codul tău de deblocare.

![Fereastra codului de deblocare: douăzeci de caractere în patru grupuri, afișate o singură dată.](assets/screenshots/offsite/recovery-code-dark.png#only-dark)
![Fereastra codului de deblocare: douăzeci de caractere în patru grupuri, afișate o singură dată.](assets/screenshots/offsite/recovery-code-light.png#only-light)

1. Pe pagina Instanțe, pornește **Păstrător de cheie** pe cardul instanței care ar trebui să o păstreze și alege **Fă depunerea**. Depunerea cere o parolă de conectare.
2. Acest server arată o singură dată **Codul de deblocare**: douăzeci de litere și cifre în patru grupuri de câte cinci. Notează-l și confirmă cu **Am notat codul**. Acest server nu stochează codul, ci doar ce îi trebuie ca să sigileze o depunere pentru el.
3. Păstrătorul ține acum depunerea, iar acest server o reînnoiește când se schimbă kitul. **Depunerea cheii** pe cardul acestei instanțe listează **Depunerea acestui server** cu fiecare păstrător și starea lui, și **Depunerile păstrate de acest server** pentru ceilalți. **Creează un cod nou** înlocuiește codul. Fiecare păstrător trece la codul nou când primește noua depunere.

În acea zi ai nevoie de trei lucruri: cele douăsprezece cuvinte ale grupului tău, codul de deblocare și accesul la interfața păstrătorului, unde cineva trebuie să apese **Permite**. În Asistentul de urgență alege **Prin grup**, alege depunerea, cere-o și tastează codul de deblocare. Codul este folosit pe noul server și nu este trimis păstrătorului. Pe păstrător, cererea apare la **Depunerea cheii** cu cine o cere, dacă acea instanță a fost văzută vreodată acolo și de câte ori a cerut. **Permite** predă copia sigilată, iar permiterea cere o parolă de conectare. Instalarea nouă are 30 de minute ca s-o ridice după Permite, o cerere la care nimeni nu răspunde expiră după o zi, iar o instanță care cere prea des trebuie să aștepte o oră.

Fără cod nimeni nu poate deschide copia, nici păstrătorul. Dacă codul se pierde, creează unul nou cât timp acest server încă rulează. Dacă serverul a dispărut deja, kitul de recuperare și `APP_KEY` tastat funcționează în continuare. Păstrătorul poate șterge o depunere oricând, iar proprietarul trebuie atunci să ceară din nou.

## Kit de recuperare {#recovery-kit}

Aceasta este piesa care face posibilă recuperarea după dezastru chiar și când nu rulează niciun BombVault.

**Descarcă kitul de recuperare** în cardul **Pentru urgențe** de la **Setări, Restaurare** îți dă **cheia**, **parola restic derivată**, fiecare **destinație** cu depozitul fiecărei secțiuni, autentificările de care are nevoie o instalare nouă ca să ajungă la ele și **comenzile restic exacte**, ca să poți restaura direct cu CLI-ul restic pe orice mașină. Cere o parolă de conectare. Kitul tipărește cheia și cu criptarea oprită, pentru că definițiile containerelor și ale VM-urilor sunt mereu sigilate cu ea. După o preluare tipărește cheia de rezervă preluată și `APP_KEY` al acestui server, de care o instalare nouă are nevoie doar pentru a readuce setările. Un card **Salvează-ți kitul de recuperare** din prezentare îți amintește până îl păstrezi.

!!! danger "Păstrează kitul de recuperare în afara serverului"
    Kitul conține secretul care decriptează copiile tale. Păstrează-l undeva în siguranță și separat de server (un manager de parole, o copie tipărită într-un seif). Dacă pierzi și BombVault, și `APP_KEY`, fără kit de recuperare și fără depunere de cheie, copiile tale criptate nu mai pot fi recuperate.

!!! warning "Cel mai nou instantaneu nu este mereu cel de restaurat"
    De la restic 0.17, `restic snapshots` arată dimensiunea fiecărui instantaneu. După o pierdere de date, cel mai nou instantaneu poate fi cel golit, așa că nu restaura un instantaneu mult mai mic decât cele dinaintea lui. După ransomware poate fi cel criptat, de dimensiunea obișnuită. Dacă BombVault încă rulează, verifică mai întâi pagina **Anomalii**: numește ultima copie bună, iar fereastra de restaurare o preselectează. O restaurare nu are nevoie de niciun dat de anomalii al BombVault, iar pauza de retenție doar păstrează mai multe instantanee.

### Gazda copiilor unui server {#backup-host}

Fiecare server își scrie copiile sub o gazdă restic proprie: `bv-` și douăsprezece caractere calculate din cheia copiilor, de exemplu `bv-3f9a0c41d2e7`. Copiile de dinainte ca un server să aibă una poartă gazda `bombvault`. Kitul numește gazda acestui server, iar comenzile din el le transmit pe amândouă cu `--host`, astfel încât `latest` să nu aleagă niciodată o copie scrisă de alt server în același depozit. Un server care preia copiile altuia continuă sub gazda acelui server, pentru că gazda vine din cheie.

### Sigilarea kitului {#sealing-the-kit}

Dacă ai pornit **Criptează exporturile cu age** la **Setări, Sistem**, și kitul este sigilat cu el și se descarcă drept `bombvault-recovery-kit.md.age`. Este în armură ASCII, nu binar, deci rămâne text simplu: lipirea lui într-un manager de parole sau tipărirea funcționează exact ca înainte, conținutul pur și simplu nu poate fi citit fără cheia ta.

!!! warning "Nu păstra cheia age în interiorul kitului"
    Ai nevoie de cheia **privată** age ca să deschizi un kit sigilat. Păstrează-o undeva care nu depinde de kitul însuși, altfel vei avea de recuperat două lucruri în loc de unul. Sigilarea merită când kitul este stocat undeva pe care nu-l controlezi pe deplin (un manager de parole partajat, notițe în cloud, o tipăritură într-un birou). Un kit din propriul seif este deja protejat de seif.

    Cu criptarea pornită și fără niciun destinatar utilizabil configurat, descărcarea este refuzată categoric. BombVault nu revine niciodată la a da cheia principală în clar.

### Când kitul nu e la îndemână {#if-you-do-not-have-the-kit-to-hand}

Parola nu este stocată nicăieri, ea este **calculată** din cheia copiilor, așa că o poți reproduce singur cu nimic altceva decât cheia și un shell:

```sh
printf 'bombvault:restic-repo'   | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r   | cut -d' ' -f1
```

Aceasta este HMAC-SHA256 peste șirul fix `bombvault:restic-repo`, cu cheia ca octeții bruți ai `APP_KEY` hex, tipărit ca 64 de caractere hex cu litere mici. Aceeași valoare este în kit, listată ca parola restic derivată. Asta este pentru ziua în care kitul e într-un loc unde nu ești. Pe un server care a preluat copiile altuia, folosește cheia de rezervă preluată, care este vechiul APP_KEY al acelui server.

[Gazda](#backup-host) vine din aceeași cheie și din șirul `bombvault:host`: `bv-` și primele douăsprezece dintre aceste caractere.

```sh
echo "bv-$(printf 'bombvault:host' | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r | cut -c1-12)"
```

!!! warning "Pentru un depozit primit, folosește cheia instanței CARE TRIMITE"
    Un depozit care a ajuns aici printr-o copie off-site a fost creat de mașina care l-a trimis, cu cheia **ei**. Derivarea din cheia stației care primește produce o parolă pe care restic o va respinge, ceea ce arată exact ca un depozit corupt și nu este. Acesta este motivul obișnuit pentru care `restic check` pe un depozit primit cere parola iar și iar.

Pentru că definițiile de recuperare se află **în interiorul** fiecărui depozit (`<repo>/def`, `<repo>/vm-def`), un folder de depozit copiat este complet autonom, deci kitul plus depozitul este tot ce are nevoie o restaurare pe hardware gol. O copie off-site și o copie preluată conțin instantaneele fără definiții: ele dau înapoi fișiere, iar un container sau un VM este recreat din Auto-backup.

## Recuperarea unui dump de bază de date {#database-dumps}

Un dump de bază de date este un punct de restaurare propriu în depozitul containerelor, etichetat `dbdump:<container>`, care conține singurul fișier `/dbdump/<container>.sql`. BombVault le listează, le descarcă și le importă în fila **Copii de rezervă** a containerului. Mai jos sunt aceiași pași doar cu restic, pentru ziua în care BombVault nu mai este.

```sh
restic -r <repo> snapshots --host <host> --host bombvault --tag dbdump:<container>
restic -r <repo> dump --host <host> --host bombvault --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

`<host>` este [gazda](#backup-host) serverului din kitul de recuperare.

Etichetele `dbversion:` și `dbname:` de pe fiecare dump spun din ce versiune de server provine și ce baze de date conține. Un fișier complet se încheie cu `-- PostgreSQL database cluster dump complete` sau `-- Dump completed`.

Importă-l într-un container de aceeași versiune sau mai nouă (PostgreSQL), sau de aceeași versiune majoră (MySQL și MariaDB), pornit o dată cu un folder de date gol, ca să se inițializeze. Gazda nu are nevoie de niciun client de bază de date, containerul are unul:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Pentru o singură bază de date dintr-un dump complet, MySQL și MariaDB acceptă `--one-database <nume>` în comanda clientului. Un dump PostgreSQL are o secțiune pe bază de date, fiecare începând cu o linie `\connect <nume>`: copiază acea secțiune într-un fișier propriu și importă-o cu `-d <nume>` după ce ai creat baza de date.

!!! warning "Un dump făcut ca root poartă utilizatorii serverului"
    Un dump complet MySQL sau MariaDB făcut ca root conține baza de date de sistem `mysql`, deci importul lui înlocuiește conturile noului server, inclusiv parola root, cu cele din dump. Pe PostgreSQL, `role ... already exists` pentru utilizatorul creat de container este de așteptat și inofensiv.
