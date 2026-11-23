# Noduri

Un nod este un BombVault fără interfață web, de exemplu pe un VPS sau pe un server mic la un prieten. Face copii de rezervă ale containerelor și folderelor de pe propriul server, iar BombVault-ul pe care îl adaugi, numit pe această pagină **instanța-mamă**, îi spune când să facă copii și unde merg ele. Îl administrezi din interfața web a instanței-mamă, lângă propriile elemente.

![Cardul nodului vps-falkenstein pe pagina Instanțe: Conectat, versiunea lui, copiile lui pe serverul receptor, când face copii fiecare secțiune, plus Schimbă destinația, Înlocuiește și Eliberează.](assets/screenshots/nodes/card-dark.png#only-dark)
![Cardul nodului vps-falkenstein pe pagina Instanțe: Conectat, versiunea lui, copiile lui pe serverul receptor, când face copii fiecare secțiune, plus Schimbă destinația, Înlocuiește și Eliberează.](assets/screenshots/nodes/card-light.png#only-light)

## Cum funcționează {#how-it-works}

- **Instanța-mamă ține cheia nodului.** Când adaugi un nod, instanța-mamă îi creează `APP_KEY`, păstrează o copie sigilată a lui și ți-l dă o singură dată, în fișierul compose al nodului. Cu această copie, instanța-mamă ajunge la nod și îi deschide copiile de rezervă.
- **Nodul nu are nevoie de niciun port deschis.** Instanța-mamă și nodul vorbesc prin relay, pe o legătură privată a lor, separată de grupul tău de împerechere. Instanța-mamă pornește fiecare apel și îl semnează, iar nodul nu cere nimic de la ea în afară de salutul lui. Singura destinație care are nevoie de un port deschis este [păstrarea copiilor pe nod](#keep-on-node).
- **Copiile de rezervă nu trec niciodată prin relay.** Nodul le trimite direct la locul ales: la serverul receptor al instanței-mamă, la una dintre destinațiile ei sau la un rest-server de pe nodul însuși.
- **Nodul nu planifică nimic singur.** Nu rulează programări, nu trimite notificări și nu intră în niciun grup și în nicio fortăreață. Instanța-mamă îi pornește copiile, urmărește cum merg și trimite alertele.
- **Un nod face copii ale containerelor și ale seturilor de foldere.** VM-urile, unitatea flash, seturile de date ZFS și auto-backupul rămân la instalările complete.

Legătura folosește relay-ul propriu al instanței-mamă când grupul ei are unul, iar altfel relay-ul proiectului, chiar și când instanța-mamă se împerechează fără relay.

## Înainte să începi {#before-you-start}

- **O parolă de conectare pe instanța-mamă.** Adăugarea unui nod, afișarea din nou a fișierului lui compose și deschiderea elementelor lui cer o autentificare cu parola, vezi [Autentificare](configuration.md#login).
- **Docker cu Compose pe serverul nodului.**
- **Un loc pentru copii**, unul dintre cele trei de la [Unde merg copiile](#placement). Serverul receptor este alegerea cea mai sigură, pentru că nodul nu poate șterge nimic acolo.

## Adăugarea unui nod {#add}

1. Pe instanța-mamă, deschide **Instanțe** și alege **Adaugă nod** jos în dreapta.
2. **Nume.** Scrie cum apare nodul aici, de exemplu numele serverului, și alege **Înainte**.

    ![Fereastra Adaugă nod la primul pas, cu numele vps-helsinki introdus.](assets/screenshots/nodes/add-name-dark.png#only-dark)
    ![Fereastra Adaugă nod la primul pas, cu numele vps-helsinki introdus.](assets/screenshots/nodes/add-name-light.png#only-light)

3. **Copii de rezervă.** Alege unde merg copiile și completează ce cere locul respectiv. **Face copii zilnic la** este ora la care pornesc ambele secțiuni. Instanța-mamă propune o jumătate de oră după copiile propriilor ei containere, ca să nu ajungă amândouă la aceleași discuri deodată. Mai târziu poți da fiecărei secțiuni propriul plan, pe card. Alege **Înainte**.

    ![Al doilea pas: serverul receptor ales, Adresă pentru noduri completată cu o adresă https și Face copii zilnic la 03:00.](assets/screenshots/nodes/add-placement-dark.png#only-dark)
    ![Al doilea pas: serverul receptor ales, Adresă pentru noduri completată cu o adresă https și Face copii zilnic la 03:00.](assets/screenshots/nodes/add-placement-light.png#only-light)

4. **Pornire.** Fereastra arată fișierul compose al nodului, cu `MODE`, `APP_KEY`, `NODE_HOME` și `NODE_NAME` completate. Copiază-l și păstrează `APP_KEY` într-un loc sigur. Pe serverul nodului:

    ```sh
    mkdir -p /opt/bombvault-node && cd /opt/bombvault-node
    nano docker-compose.yml   # lipește fișierul din fereastră
    docker compose up -d
    ```

    Înainte să-l pornești, îndreaptă volumul Host Data spre datele pe care vrei să le salvezi și setează `HOST_SOURCE_ROOT` la aceeași cale, cum arată exemplul de mai jos.

5. Fereastra așteaptă nodul. Când nodul răspunde, instanța-mamă îi spune unde merg copiile lui și verifică dacă ajunge la ele, iar fereastra spune **vps-helsinki e online și primește comenzi de la această instanță.**

    ![Ultimul pas: fișierul compose, apoi nodul online și copiile lui așezate pe serverul receptor.](assets/screenshots/nodes/add-start-dark.png#only-dark)
    ![Ultimul pas: fișierul compose, apoi nodul online și copiile lui așezate pe serverul receptor.](assets/screenshots/nodes/add-start-light.png#only-light)

Poți închide fereastra înainte să răspundă nodul. Cardul lui arată atunci **Se așteaptă nodul**, iar instanța-mamă îi spune unde merg copiile de îndată ce apare.

### Exemplu: un VPS care face copii pe serverul receptor {#example-vps}

Instanța-mamă este o stație Unraid numită `tower`, care rulează [serverul receptor](offsite-recovery.md#receiving-server). Un reverse proxy îl publică drept `https://backup.tower.example.org`. VPS-ul `vps-helsinki` rulează Nextcloud și Caddy, cu datele lor în `/srv/appdata`.

1. Pe `tower`: **Instanțe**, **Adaugă nod**, numele `vps-helsinki`, **Înainte**.
2. Alege **Serverul receptor al acestei instanțe** și scrie `https://backup.tower.example.org` la **Adresă pentru noduri**. Containerele instanței-mamă fac copii la 02:30, așa că fereastra propune 03:00. Păstrează ora și alege **Înainte**.
3. Pe VPS, salvează fișierul din fereastră, cu volumele și `HOST_SOURCE_ROOT` schimbate pentru `/srv/appdata`:

    ```yaml
    services:
      bombvault-node:
        image: junkerderprovinz/bombvault:latest
        restart: unless-stopped
        hostname: bombvault-node
        container_name: bombvault-node
        volumes:
          - ./config:/config
          - /var/run/docker.sock:/var/run/docker.sock
          - /srv/appdata:/host/user
        environment:
          MODE: node
          APP_KEY: "<filled in by home>"
          NODE_HOME: "<filled in by home>"
          NODE_NAME: "vps-helsinki"
          PLATFORM: generic
          HOST_SOURCE_ROOT: /srv/appdata
    ```

4. `docker compose up -d`. Câteva secunde mai târziu, fereastra de pe `tower` spune că nodul e online și **Copiile de rezervă merg la https://backup.tower.example.org.**
5. La 03:00 instanța-mamă pornește copiile nodului. A doua zi dimineață, cardul **Noduri** din prezentare arată ultima copie a lui `vps-helsinki`, iar **Copii de rezervă** îi listează containerele într-o secțiune proprie.

Cum sunt găsite datele unui container pe o gazdă Docker obișnuită este descris la [Detectarea surselor de backup](configuration.md#backup-source-detection).

## Unde merg copiile {#placement}

Fiecare nod are o singură destinație pentru ambele secțiuni. **Schimbă destinația** de pe cardul lui o schimbă după ce nodul e online. Locul nou primește depozite noi, iar copiile din locul vechi rămân unde sunt.

### Serverul receptor al acestei instanțe {#to-receiver}

Nodul scrie pe serverul receptor al instanței-mamă în mod append-only: poate adăuga copii, dar nu poate șterge niciuna. Primește acolo o autentificare proprie, numită după nod, și câte un depozit pe secțiune, în propriul folder. Instanța-mamă își adaugă cheia în fiecare depozit și face singură curățenie în ele, pentru că nodul nu poate.

Dacă instanța-mamă nu rulează încă un server receptor, pasul spune asta și oferă **Configurează serverul receptor**. Instanța-mamă nu are nevoie de niciun grup pentru asta. Alege un folder pe o partajare și un port, apasă **Configurează și continuă**, apoi **Gata** când fereastra arată serverul, iar pasul continuă cu el. Detaliile sunt la [Server receptor](offsite-recovery.md#receiving-server).

Nodul ajunge la server la **Adresă pentru noduri**, pe care o setezi la al doilea pas sau în fereastra **Server receptor**. De obicei un nod trimite din internet, așa că adresa trebuie să fie una dintre acestea:

- `https://`, de exemplu în spatele unui reverse proxy, al lui Pangolin sau al unui tunel. Când serverul arată un certificat propriu în locul unuia de la o autoritate publică, lipește-l în format PEM la **Certificat**.
- `http://` cu o adresă IP privată, de exemplu `10.0.0.5` pe o legătură WireGuard sau `100.101.102.103` în Tailscale.

Un `http://` simplu spre o adresă publică este refuzat, pentru că nodul și-ar trimite copiile prin internet necriptate. Când schimbi adresa, instanța-mamă anunță fiecare nod de pe receptor, iar un nod care lipsește află când revine.

### O destinație {#to-location}

Nodul scrie într-una dintre destinațiile instanței-mamă la care ajunge el însuși: un serviciu S3 sau un server REST, prin `https://` sau la o adresă privată. Depozitele lui se află acolo în `bombvault-node-<node id>-containers` și `bombvault-node-<node id>-files`.

Nodul se conectează cu o [autentificare salvată](offsite-recovery.md#sign-ins) făcută pentru el, care trebuie să fie diferită de cea cu care scrie instanța-mamă în destinație. Adaug-o întâi dacă fereastra spune că nu există niciuna. Cu această autentificare, nodul poate de obicei să șteargă ce se află în destinație, spre deosebire de serverul receptor, iar fereastra îți spune asta.

### Păstrează-le pe nod {#keep-on-node}

Nodul își păstrează singur copiile, în spatele unui rest-server propriu în mod append-only, iar instanța-mamă preia o copie a fiecărei secțiuni în propria stocare. Se potrivește unui nod căruia instanța-mamă nu îi poate oferi un loc. Este singura destinație care are nevoie de un port deschis.

![Al doilea pas cu Păstrează-le pe nod ales, adresa https://203.0.113.7:8443 și nota despre portul deschis.](assets/screenshots/nodes/add-keep-dark.png#only-dark)
![Al doilea pas cu Păstrează-le pe nod ales, adresa https://203.0.113.7:8443 și nota despre portul deschis.](assets/screenshots/nodes/add-keep-light.png#only-light)

1. La **Adresa nodului**, scrie unde ajunge instanța-mamă la nod: adresa lui publică și un port liber, de exemplu `https://203.0.113.7:8443`, fără niciun folder după el. Folosește `http://` doar cu o adresă privată, ca mai sus.
2. Nodul pornește containerul `bombvault-node-rest-server` pe acel port, cu depozitele în folderul `bombvault-node` din Host Data. Cu `https://`, face un certificat pentru acea adresă, iar instanța-mamă îl fixează.
3. Lasă portul să treacă prin firewall-ul nodului, ca instanța-mamă să ajungă la el. Un port ținut de alt container de pe nod este refuzat, cu numele acelui container.

Instanța-mamă preia o secțiune imediat după ce se termină copiile ei pe nod, iar ca rezervă în fiecare oră, la și un sfert, chiar și cu **Preluare** oprită. Ce a preluat rămâne la instanța-mamă ca orice [copie preluată](offsite-recovery.md#receiver), chiar și după ce nodul dispare. Nodul își curăță singur depozitele când i se cere, pentru că rest-server-ul lui nu acceptă ștergeri, iar seturile lui de foldere lasă deoparte folderul `bombvault-node`, ca să nu-și copieze niciodată propriile copii.

### Copii off-site împreună cu cele ale instanței-mamă {#copy-onward}

Pornește **Copiază în afara locației împreună cu copiile acestei instanțe** la pasul **Copii de rezervă** sau la **Schimbă destinația**, iar instanța-mamă copiază containerele și seturile de foldere ale nodului la destinațiile la care copiază propriile ei Containere și Foldere, după același plan. Nodul nu are nevoie pentru asta de un folder propriu sau de un plan de copiere propriu.

- Copiile stau în aceleași depozite ca și copiile instanței-mamă, sub gazda nodului. Regulile instanței-mamă de la destinație le lasă în pace, iar instanța-mamă le păstrează după regula nodului, aceeași după care [face curățenie în copiile nodului](#schedule).
- Instanța-mamă citește depozitele nodului cu propria ei cheie și scrie cu propriile ei credențiale pentru destinație. Nodul nu primește credențiale noi, iar serverul receptor rămâne append-only. La un nod care își păstrează singur copiile, instanța-mamă copiază ce a preluat.
- Depozitele nodului au setări de chunker proprii, așa că copiile lor împart puține date cu cele ale instanței-mamă la destinație.
- La o destinație append-only nu se face curățenie, ca la copiile proprii ale instanței-mamă. Un [depozit comun](offsite-recovery.md#shared-repository) nu primește copii ale unui nod, pentru că proprietarul lui face curățenie în el după regulile serverelor grupului.
- Cardul arată **Copiile merg la** și destinațiile. O copie care eșuează este o eroare pe card și trimite o alertă, vezi [Urmărirea nodurilor](#watch).
- Oprit, instanța-mamă nu mai copiază nimic. Copiile care există deja rămân, iar regulile instanței-mamă le lasă în continuare în pace.

#### Exemplu: un VPS copiat în B2 împreună cu containerele instanței-mamă {#example-copy-onward}

`tower` păstrează la **Cât timp se păstrează** 7 copii ale fiecărui container și își copiază Containerele într-un bucket B2 în fiecare noapte la 04:00. Bucket-ul păstrează ultimele 14. `vps-helsinki` face copii pe serverul receptor al lui `tower`, unde se face curățenie în copiile lui după aceleași 7. Cu comutatorul pornit, `tower` copiază la 04:00 și containerele VPS-ului în bucket. Bucket-ul ține atunci 14 copii ale fiecărui container de pe `tower` și 7 ale fiecărui container al VPS-ului.

## Când fac nodurile copii {#schedule}

Programarea fiecărui nod este a instanței-mamă. Cardul arată un plan pentru fiecare secțiune, de exemplu **Containere** *zilnic la 2:00* și **Foldere** *săptămânal (dum.) la 4:00*. Un clic deschide același editor ca la [Programări](schedules.md#plans), iar o schimbare se salvează imediat.

![Planul containerelor unui nod, deschis de pe cardul lui, cu Oprit, Zilnic, Săptămânal, La fiecare N zile și Cron.](assets/screenshots/nodes/schedule-dark.png#only-dark)
![Planul containerelor unui nod, deschis de pe cardul lui, cu Oprit, Zilnic, Săptămânal, La fiecare N zile și Cron.](assets/screenshots/nodes/schedule-light.png#only-light)

- **Un nod care a lipsit** pornește copiile ratate imediat ce revine. Instanța-mamă verifică la fiecare cinci minute.
- **Un nod ocupat** este întrebat din nou cinci minute mai târziu.
- **Cât timp instanța-mamă face curățenie** în copiile vechi ale unui nod, nodul nu pornește nimic până nu termină ea. Pe serverul receptor și pe nodul însuși, această fereastră se deschide în fiecare duminică la 03:00 și durează cel mult șase ore. La o destinație, urmează planul de copii off-site al instanței-mamă pentru acea destinație. Instanța-mamă păstrează copiile după propria regulă a secțiunii la **Cât timp se păstrează** sau după regula destinației, acolo unde are una.
- **În lista de rulări a nodului**, copiile și restaurările pornite de instanța-mamă apar ca pornite de instanța-mamă.

## Urmărirea nodurilor {#watch}

Cardul **Noduri** din prezentare listează fiecare nod cu starea lui, ultima copie, erorile și anomaliile deschise. Apare de îndată ce ai un nod.

![Cardul Noduri din prezentare cu trei noduri: unul conectat, unul inactiv cu două erori și unul care încă așteaptă nodul.](assets/screenshots/nodes/overview-card-dark.png#only-dark)
![Cardul Noduri din prezentare cu trei noduri: unul conectat, unul inactiv cu două erori și unul care încă așteaptă nodul.](assets/screenshots/nodes/overview-card-light.png#only-light)

Instanța-mamă trimite o singură alertă pe episod prin [canalele ei de notificare](notifications.md), iar următoarea abia după ce acel episod s-a încheiat:

- **O copie a eșuat**, de exemplu *BombVault: vps-helsinki: backup failed*, cu elementul și motivul. O copie ulterioară reușită a acelui element încheie episodul.
- **Nodul nu poate fi contactat**, de exemplu *BombVault: vps-helsinki is out of reach*. Instanța-mamă așteaptă cât cel mai scurt comutator de om mort de pe depozitele nodului, 26 de ore dacă nu îl schimbi, sau 6 ore cât timp nodul nu are încă o destinație.
- **O copie off-site a eșuat**, de exemplu *BombVault: vps-helsinki: off-site copy failed*, când instanța-mamă nu a putut copia copiile nodului la una dintre destinațiile ei. Următoarea copie care reușește încheie episodul.

Supravegherea receptorului pe instanța-mamă se ocupă și de fiecare depozit al unui nod, cu un comutator de om mort și un `restic check` zilnic la 04:00, vezi [Receptor și preluator](offsite-recovery.md#receiver).

## Elementele unui nod {#entries}

La **Copii de rezervă**, listele Containere și Foldere au câte o secțiune pentru fiecare nod, sub propriile tale elemente. Containerul propriu al nodului este lăsat deoparte. **Copiază acum** din meniul unui element îi pornește copia pe nod, iar **Deschide** îi deschide pagina.

În secțiunea unui nod din lista Foldere, **Adaugă set de foldere** adaugă un set de foldere pe nod, iar **Editează** din meniul unui set îl schimbă. Ambele deschid aceeași fereastră ca pentru propriile tale seturi. Selectorul de foldere din ea răsfoiește folderele nodului, dar nu creează foldere noi acolo.

![Secțiunea nodului vps-falkenstein din lista Containere: nextcloud, nextcloud-db, caddy și uptime-kuma.](assets/screenshots/nodes/backups-section-dark.png#only-dark)
![Secțiunea nodului vps-falkenstein din lista Containere: nextcloud, nextcloud-db, caddy și uptime-kuma.](assets/screenshots/nodes/backups-section-light.png#only-light)

Pagina unui element de pe nod arată **Pe** și numele nodului și îi listează copiile. **Restaurare** la una dintre ele restaurează elementul pe nod și înlocuiește ce se află acolo acum. Un container este oprit pentru asta, ca la instanța-mamă.

![Pagina lui nextcloud pe vps-falkenstein, cu copiile lui și un buton Restaurare lângă fiecare.](assets/screenshots/nodes/entry-dark.png#only-dark)
![Pagina lui nextcloud pe vps-falkenstein, cu copiile lui și un buton Restaurare lângă fiecare.](assets/screenshots/nodes/entry-light.png#only-light)

Prin instanța-mamă ajungi la o listă fixă de rute proprii ale nodului: containerele, seturile de foldere, rulările și anomaliile lui, plus copierea și restaurarea lor. Setările, cheile și stocarea nodului rămân în afara accesului, iar nimic de pe acea listă nu șterge. Vezi [Noduri în API](api.md#nodes).

## Înlocuire și eliberare {#replace-release}

**Înlocuiește** arată din nou fișierul compose al nodului, după o autentificare cu parola. Folosește-l ca să muți nodul pe alt server sau ca să-l refaci: pornește fișierul acolo și oprește serverul vechi. Același `APP_KEY` și același `NODE_HOME` îl fac din nou același nod, cu copiile și istoricul lui. Instanța-mamă păstrează pentru fiecare nod seturile de foldere, containerele cu tot ce trebuie ca să fie refăcute și rulările. Când nodul pornește cu un folder `config` gol, instanța-mamă îl preia din nou, îi spune unde merg copiile lui și îi dă înapoi toate acestea înainte de următoarea copie, așa că nodul poate restaura un container care nu mai există. Pe un server nou, un nod care își păstrează singur copiile pornește cu depozite goale. Copiile lui de dinainte rămân în cele preluate de instanța-mamă. Două servere cu același `APP_KEY` contează ca un singur nod, așa că nu le rula niciodată pe amândouă.

**Eliberează** lasă un nod să plece. Nu mai primește comenzi de la instanța-mamă și nu mai face nicio copie. Copiile lui rămân unde sunt, la fel ca fiecare copie preluată de instanța-mamă. Autentificarea lui pe serverul receptor dispare, iar instanța-mamă nu-i mai urmărește depozitele. Un nod care lipsește când îl eliberezi își păstrează cardul ca **Se eliberează** până află. Un nod eliberat nu mai acceptă nicio instanță-mamă cu același `APP_KEY`. Ca să folosești din nou serverul, adaugă-l ca nod nou și pornește-l cu noul fișier compose și un folder `config` gol.

## Securitate {#security}

Dacă instanța-mamă este preluată, atacatorul îi controlează și nodurile, iar un nod ține Docker-ul serverului lui, adică root acolo. Protejează instanța-mamă cu o parolă și ține-o departe de internet. Dacă un nod este preluat, atacatorul nu obține grupul de împerechere, fortăreața, alt nod sau alt depozit și nu poate șterge istoricul de pe serverul receptor. Detaliile sunt la [Noduri în modelul de securitate](configuration.md#nodes).

[Kitul de recuperare](offsite-recovery.md#recovery-kit) are un capitol **Noduri** cu `APP_KEY` al fiecărui nod, parola lui restic și depozitele lui, ca să poți readuce un nod sau să-i restaurezi copiile cu restic simplu.
