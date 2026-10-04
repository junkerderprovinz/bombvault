# Aplicația Android

Aplicația Android pune pe telefon toate serverele BombVault din grupul tău. Pornește cu o listă a serverelor tale, cu jurnalul de activitate al tuturor deasupra, aceleași rânduri pe care le arată panoul principal, și deschide vizualizarea pentru telefon a serverului pe care îl atingi. Aplicația nu face ea însăși niciun backup.

## Obținerea aplicației {#install}

- **APK:** fiecare versiune are `bombvault-android.apk` pe pagina sa de release, iar [acest link](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) descarcă mereu cea mai nouă versiune. Are nevoie de Android 10 sau mai nou, iar Android întreabă o dată dacă aplicația cu care deschizi fișierul are voie să instaleze aplicații.
- **Google Play:** aplicația este într-un test închis până când poate deveni publică. Google Play listează o aplicație de la un cont de dezvoltator nou doar după ce cel puțin 12 testeri au păstrat-o instalată timp de 14 zile. Ca să ajuți, intră în [grupul de testeri](https://groups.google.com/g/arrowloop-testers), deschide [pagina de test](https://play.google.com/apps/testing/bombvault.halleluja.design), atinge **Deveniți tester** și instalează BombVault din Google Play.
- **F-Droid:** listarea va urma.

Împerechează aplicația cu servere la versiunea 9.7.0 sau mai nouă. Un server cu o versiune mai veche poate fi în același grup, dar afișează telefonul ca pe o instanță obișnuită, iar aplicația citește activitatea acelui server doar după o autentificare.

## Împerechere prin cod QR {#pairing}

1. Pe oricare server din grupul tău, deschide **Setări, Împerechere** și alege **Arată fraza**. Cele douăsprezece cuvinte apar cu un cod QR alături.
2. În aplicație, atinge **Scanează cod QR** și îndreaptă telefonul spre cod. Poți și să lipești sau să tastezi cuvintele.
3. Aplicația listează serverele din acel grup înainte să salveze ceva. **Adaugă toate cele N** le adaugă pe toate.

Telefonul intră apoi în grup ca încă o instanță. Citește ce rulează pe fiecare server prin grup, fără autentificare, direct când ești acasă și prin releu când ești plecat. Felul în care funcționează grupul însuși este descris la [Împerecherea instanțelor](offsite-recovery.md#pairing).

!!! note "Interfața tot are nevoie de o cale până la server"
    Lista de servere și jurnalul de activitate vin prin grup. Interfața unui server se deschide direct, așa că telefonul trebuie să poată ajunge la adresa serverului, acasă sau printr-un VPN.

## Autentificat pe un telefon împerecheat {#sign-in}

Un telefon împerecheat cu grupul tău deschide fiecare dintre serverele lui deja autentificat. Înainte să încarce o pagină, cere acelui server o sesiune prin grup, iar un server acordă una doar unui membru care este telefon. Un server adăugat după adresă cere parola, ca într-un browser. Oricine are cele douăsprezece cuvinte poate deja să deschidă orice backup al grupului, așa că împerecherea nu acordă nimic nou.

## Servere din afara unui grup {#other-servers}

- **Adaugă server** primește adresa cu care deschizi BombVault într-un browser, de exemplu `192.168.1.10:3443`. Fără `http://` sau `https://` în față, aplicația folosește https.
- Serverele care se anunță în rețeaua locală apar la **În această rețea** și se deschid cu o atingere. Fac asta cât timp **Găsește în rețea** este activat la Setări, Integrări. Un server dintr-o altă rețea sau din spatele unui VPN nu apare acolo.
- Un certificat autosemnat este acceptat o dată, după amprenta sa SHA-256. Când serverul arată mai târziu un alt certificat, aplicația te avertizează și îl deschide doar după ce ai acceptat noul certificat.

## Telefonul pe pagina Instanțe {#instances}

Telefonul primește propriul card pe pagina Instanțe a fiecărui server din grup, marcat ca aplicație Android și numit după numele pe care l-ai dat telefonului. Nu are fișă de evaluare, pentru că nu face backup la nimic, iar **Elimină** îl scoate de pe pagină.

## Setări {#settings}

Rotița de lângă butonul + deschide setările aplicației:

- limba și numele pe care telefonul îl arată pe pagina Instanțe (gol înseamnă modelul telefonului),
- aspectul, care urmează primul server din listă până când îl setezi pe al tău, și animațiile, care au o setare proprie,
- un raport de copiat când raportezi o problemă; nu conține nicio adresă, niciun nume și nicio frază,
- cardul **Despre** cu politica de confidențialitate,
- **Elimină toate serverele**, care scoate toate serverele din aplicație și părăsește grupul. Pe servere nu se schimbă nimic.

## Descărcări și încărcări {#files}

Exporturile, kiturile de recuperare, arhivele ZIP ale flash-ului și dumpurile de baze de date ajung în dosarul Descărcări al telefonului, ca dintr-un browser. Un import de setări deschide selectorul de fișiere al telefonului.

## Pe un ecran tactil {#touch}

Sub un deget nu există hover, așa că un control se estompează cât timp este ținut apăsat, iar un buton cu logoul unei mărci se aprinde în culoarea acelei mărci până când degetul se ridică. O apăsare lungă pe un buton contează ca o atingere lentă și nu deschide niciun meniu de link.

## Sau într-un browser {#browser}

Chrome și Edge pot instala interfața web BombVault ca aplicație în propria fereastră, pe telefon la fel ca pe calculator. Nimic nu este păstrat în cache, așa că o actualizare apare imediat.

## Confidențialitate {#privacy}

Aplicația nu are conturi, publicitate sau statistici de utilizare și nu rulează nimic în fundal. [Politica de confidențialitate](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) a aplicației arată ce stochează și ce trimite și unde.
