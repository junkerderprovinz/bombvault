# Aplicația Android

Aplicația Android pune pe telefon toate serverele BombVault din grupul tău. Pornește cu o listă a serverelor tale, cu jurnalul de activitate al tuturor deasupra, aceleași rânduri pe care le arată prezentarea, și deschide vizualizarea pentru telefon a serverului pe care îl atingi. Aplicația nu face ea însăși niciun backup.

## Obținerea aplicației {#install}

- **Setări, Aplicații:** cardul aplicației Android oferă APK-ul pentru versiunea pe care o rulează serverul tău, cu un cod QR de scanat de pe telefon.
- **APK:** fiecare versiune are `bombvault-android.apk` pe pagina sa de release, iar [acest link](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) descarcă mereu cea mai nouă versiune. Are nevoie de Android 10 sau mai nou, iar Android întreabă o dată dacă aplicația cu care deschizi fișierul are voie să instaleze aplicații.
- **Google Play:** aplicația este într-un test închis până când poate deveni publică. Google Play listează o aplicație de la un cont de dezvoltator nou doar după ce cel puțin 12 testeri au păstrat-o instalată timp de 14 zile. Ca să ajuți, intră în [grupul de testeri](https://groups.google.com/g/arrowloop-testers), deschide [pagina de test](https://play.google.com/apps/testing/bombvault.halleluja.design), atinge **Deveniți tester** și instalează BombVault din Google Play.
- **F-Droid:** listarea va urma.

Împerechează aplicația cu servere la versiunea 9.7.0 sau mai nouă. Un server cu o versiune mai veche poate fi în același grup, dar afișează telefonul ca pe o instanță obișnuită, iar aplicația citește activitatea acelui server doar după o autentificare.

## Împerechere prin cod QR {#pairing}

1. Pe oricare server din grupul tău, deschide **Setări, Împerechere** și alege **Arată fraza**. Cele douăsprezece cuvinte apar cu un cod QR alături.
2. În aplicație, atinge **Scanează cod QR** și îndreaptă telefonul spre cod. Poți și să lipești sau să tastezi cuvintele.
3. Aplicația listează serverele din acel grup înainte să salveze ceva. **Adaugă toate cele N** le adaugă pe toate.

Telefonul intră apoi în grup ca încă o instanță. Citește ce rulează pe fiecare server prin grup, fără autentificare, direct când ești acasă și prin releu când ești plecat. Felul în care funcționează grupul însuși este descris la [Împerecherea instanțelor](offsite-recovery.md#pairing).

Aplicația intră în grup prin releu, și acasă. Dacă instanța ta este pe **Fără relay**, alege unul în **Setări, Împerechere** înainte să scanezi codul.

## În afara casei {#away}

Aplicația încearcă mai întâi serverul la adresa lui proprie. Când aceasta nu răspunde, de exemplu în rețeaua mobilă, deschide în schimb interfața serverului prin releul grupului, iar un rând sub pagină spune **Deschis prin relay**. Un server al grupului a cărui adresă aplicația încă nu o cunoaște se deschide la fel. Dacă nu răspunde nici adresa, nici releul, ecranul de pornire arată asta pe cardul serverului.

Prin releu, pagina ajunge la ce ajunge un telefon împerecheat acasă, așa cum este descris mai jos. Autentificarea cu parola și salvarea fișierelor funcționează doar la adresa proprie a serverului. Barele de progres se actualizează o dată la două secunde, nu continuu. Fișierele interfeței rămân pe telefon după prima vizită, deci vizitele următoare se încarcă mai repede. Un server mai vechi de 10.0.0 se deschide doar la adresa lui proprie.

Aplicația acceptă un răspuns doar dacă serverul l-a semnat cu cheia pe care aplicația a fixat-o la prima întâlnire cu acel server. Dacă serverul răspunde mai târziu cu altă cheie, ecranul de pornire arată amprenta noii chei, iar pagina se deschide doar după ce ai acceptat-o.

## Autentificat pe un telefon împerecheat {#sign-in}

Un telefon împerecheat cu grupul tău deschide fiecare dintre serverele lui deja autentificat. Înainte să încarce o pagină, cere acelui server o sesiune prin grup, iar un server acordă una doar unui membru care este telefon. Un server adăugat după adresă cere parola, ca într-un browser. 

Un telefon împerecheat nu primește tot ce primește o parolă. Sesiunea lui ajunge la o listă fixă din ce arată aplicația: prezentarea, rulările și jurnalul de activitate, copiile de rezervă de privit, pornirea și restaurarea, anomaliile de citit și pagina Instanțe de privit. Destinațiile și comenzile ajung la el fără adrese și fără secrete. O restaurare care înlocuiește originalul face mai întâi o copie de rezervă a stării curente. Cheile, kitul de recuperare, depunerile de chei, setările și ștergerea copiilor de rezervă rămân la o autentificare chiar pe server, iar orice altceva răspunde că are nevoie de acea autentificare. Prin releu se aplică aceeași listă, chiar și pe un server fără parolă.

Comutatorul pentru asta este **Telefoanele se pot împerechea cu cele douăsprezece cuvinte ale grupului**, în cardul **Aplicația Android** de la **Setări, Securitate**. O resetare a parolei îl oprește și deconectează toate telefoanele, până când îl pornești la loc acolo.

## Servere din afara unui grup {#other-servers}

- **Adaugă server** primește adresa cu care deschizi BombVault într-un browser, de exemplu `192.168.1.10:3443`. Fără `http://` sau `https://` în față, aplicația folosește https.
- Serverele care se anunță în rețeaua locală apar la **În această rețea** și se deschid cu o atingere. Fac asta cât timp **Găsește în rețea** este activat în cardul **Servicii** de la **Setări, Integrări**. Un server dintr-o altă rețea sau din spatele unui VPN nu apare acolo.
- Un certificat autosemnat este acceptat o dată, după amprenta sa SHA-256. Când serverul arată mai târziu un alt certificat, aplicația te avertizează și îl deschide doar după ce ai acceptat noul certificat.

## Telefonul pe pagina Instanțe {#instances}

Telefonul primește propriul card pe pagina Instanțe a fiecărui server din grup, marcat ca aplicație Android și numit după numele pe care l-ai dat telefonului. Nu are fișă de evaluare, pentru că nu face backup la nimic, iar **Elimină** îl scoate de pe pagină. Un telefon care cunoaște cuvintele se poate împerechea din nou oricând, așa că pentru un telefon pierdut dezactivează **Telefoanele se pot împerechea cu cele douăsprezece cuvinte ale grupului** la **Setări, Securitate** sau lasă fiecare instanță să aleagă **Părăsește grupul** și împerechează-le din nou cu cuvinte noi.

## Setări {#settings}

Rotița de lângă butonul + deschide setările aplicației:

- limba și numele pe care telefonul îl arată pe pagina Instanțe (gol înseamnă modelul telefonului),
- aspectul, care urmează primul server din listă până când îl setezi pe al tău, și animațiile, care au o setare proprie,
- un raport de copiat când raportezi o problemă; nu conține nicio adresă, niciun nume și nicio frază,
- cardul **Despre** cu politica de confidențialitate,
- **Elimină toate serverele**, care scoate toate serverele din aplicație și părăsește grupul. Pe servere nu se schimbă nimic.

## Descărcări și încărcări {#files}

Exporturile, kiturile de recuperare, arhivele ZIP ale flash-ului și dumpurile de baze de date ajung în dosarul Descărcări al telefonului, ca dintr-un browser. Un import de setări deschide selectorul de fișiere al telefonului. Prin releu nu se salvează niciun fișier; pentru asta deschide serverul la adresa lui proprie.

## Pe un ecran tactil {#touch}

Sub un deget nu există hover, așa că un control se estompează cât timp este ținut apăsat, iar un buton cu logoul unei mărci se aprinde în culoarea acelei mărci până când degetul se ridică. O apăsare lungă pe un buton contează ca o atingere lentă și nu deschide niciun meniu de link.

## Sau într-un browser {#browser}

Chrome și Edge pot instala interfața web BombVault ca aplicație în propria fereastră, pe telefon la fel ca pe calculator. Nimic nu este păstrat în cache, așa că o actualizare apare imediat.

## Confidențialitate {#privacy}

Aplicația nu are conturi, publicitate sau statistici de utilizare și nu rulează nimic în fundal. [Politica de confidențialitate](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) a aplicației arată ce stochează și ce trimite și unde.
