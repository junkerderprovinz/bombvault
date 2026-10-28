# Android-app

De Android-app zet elke BombVault-server van je groep op je telefoon. Hij start met een lijst van je servers met bovenaan het activiteitenlogboek van allemaal, dezelfde regels als in het overzicht, en opent de telefoonweergave van de server die je aantikt. De app maakt zelf geen back-ups.

## De app downloaden {#install}

- **Instellingen, Apps:** de kaart van de Android-app biedt de APK aan voor de release die je server draait, met een QR-code om vanaf de telefoon te scannen.
- **APK:** elke release heeft `bombvault-android.apk` op de releasepagina, en [deze link](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) downloadt altijd de nieuwste build. De app heeft Android 10 of nieuwer nodig, en Android vraagt één keer of de app waarmee je het bestand opent apps mag installeren.
- **Google Play:** de app zit in een gesloten test tot hij openbaar kan worden. Google Play toont een app van een nieuw ontwikkelaarsaccount pas nadat minstens 12 testers hem 14 dagen lang geïnstalleerd hebben gehouden. Wil je helpen, word dan lid van de [testersgroep](https://groups.google.com/g/arrowloop-testers), open de [testpagina](https://play.google.com/apps/testing/bombvault.halleluja.design), tik op **Tester worden** en installeer BombVault vanuit Google Play.
- **F-Droid:** [BombVault op F-Droid](https://f-droid.org/packages/bombvault.halleluja.design/). F-Droid bouwt de app zelf uit de broncode en ondertekent hem met een eigen sleutel, dus een nieuwe release komt daar een paar dagen later aan. Wil je wisselen tussen de F-Droid-app en de APK, verwijder dan eerst de andere.

Koppel de app met servers op versie 9.7.0 of nieuwer. Een server op een oudere versie kan in dezelfde groep zitten, maar toont de telefoon als een gewone instantie, en de app leest de activiteit van die server pas na aanmelden.

## Koppelen via QR-code {#pairing}

1. Open op een willekeurige server van je groep **Instellingen, Koppeling** en kies **Frase tonen**. De twaalf woorden verschijnen met een QR-code ernaast.
2. Tik in de app op **QR-code scannen** en richt de telefoon op de code. Je kunt de woorden ook plakken of typen.
3. De app toont de servers van die groep voordat hij iets opslaat. **Alle toevoegen** voegt ze allemaal toe.

De telefoon treedt daarna tot de groep toe als een extra instantie. Hij leest via de groep zonder aanmelden wat er op elke server draait, thuis rechtstreeks en onderweg via de relay. Hoe de groep zelf werkt, staat bij [Koppeling van instanties](offsite-recovery.md#pairing).

De app treedt via de relay tot de groep toe, ook thuis. Staat je instantie op **Geen relay**, kies er dan een onder **Instellingen, Koppeling** voordat je de code scant.

## Onderweg {#away}

De app probeert een server eerst op zijn eigen adres. Antwoordt dat niet, bijvoorbeeld op het mobiele netwerk, dan opent ze de interface van de server via de relay van de groep. Een server van de groep waarvan de app het adres nog niet kent, opent op dezelfde manier. Antwoorden noch het adres noch de relay, dan meldt het startscherm dat op de kaart van de server.

Via de relay bereikt de pagina wat een gekoppelde telefoon thuis bereikt, zoals hieronder beschreven. Aanmelden met het wachtwoord en bestanden opslaan werken alleen op het eigen adres van de server. Voortgangsbalken worden elke twee seconden bijgewerkt in plaats van doorlopend. De bestanden van de interface blijven na het eerste bezoek op de telefoon, dus latere bezoeken laden sneller. Een server ouder dan 10.0.0 opent alleen op zijn eigen adres.

De app neemt een antwoord alleen aan als de server het heeft ondertekend met de sleutel die de app vastpinde toen ze die server voor het eerst tegenkwam. Antwoordt de server later met een andere sleutel, dan toont het startscherm de vingerafdruk van de nieuwe sleutel en opent de pagina pas nadat je die vertrouwt.

## Van server wisselen {#switch}

Is een server open, dan staat bovenaan zijn overzicht zijn naam, met een pijl terug ervoor. De pijl leidt naar het overzicht van de app, het startscherm, dat elke server die de app kent met zijn status toont: **Verbonden**, **Niet verbonden**, **Aanmelden nodig** of **Certificaat controleren**. Een server van de groep die niet antwoordt, staat op **Inactief**, in grijs, met wanneer de groep hem voor het laatst zag, en een server waarvan de app nog niets heeft gehoord, toont **Controleren**. Bij elke verbinding leest de app de lijst van de groep bij een van de instanties, dus staat ook een instantie die de app nooit via de relay heeft gezien in de lijst, zoals een die uit stond toen de telefoon koppelde. Tik daar op een andere om hem te openen, thuis rechtstreeks en anders via de relay.

## Aangemeld op een gekoppelde telefoon {#sign-in}

Een telefoon die met je groep is gekoppeld, opent elke server van die groep al aangemeld. Voordat hij een pagina laadt, vraagt hij die server via de groep om een sessie, en een server geeft er alleen een aan een lid dat een telefoon is. Een server die je via zijn adres hebt toegevoegd, vraagt net als in een browser om het wachtwoord.
Een gekoppelde telefoon krijgt niet alles wat een wachtwoord geeft. Zijn sessie bereikt een vaste lijst van wat de app toont: het overzicht, de runs en het activiteitenlogboek, de back-ups om te bekijken, te starten en te herstellen, de anomalieën om te lezen en de pagina Instanties om te bekijken. Bestemmingen en commando's komen terug zonder hun adressen en geheimen. Een herstel dat het origineel vervangt, maakt eerst een back-up van de huidige staat. Sleutels, de herstelkit, bewaarkopieën van de sleutel, instellingen en het verwijderen van back-ups blijven bij een aanmelding op de server zelf, en al het andere antwoordt dat het die aanmelding nodig heeft. Via de relay geldt dezelfde lijst, ook op een server zonder wachtwoord.

De schakelaar hiervoor is **Telefoons kunnen koppelen met de twaalf woorden van de groep** in de kaart **Android-app** onder **Instellingen, Beveiliging**. Een wachtwoordreset zet hem uit en meldt alle telefoons af, totdat je hem daar weer aanzet.

## Servers buiten een groep {#other-servers}

- **Server toevoegen** neemt het adres waarmee je BombVault in een browser opent, zoals `192.168.1.10:3443`. Zonder `http://` of `https://` ervoor gebruikt de app https.
- Servers die zich op het lokale netwerk bekendmaken, staan onder **In dit netwerk** en openen met één tik. Dat doen ze zolang **Vinden op het netwerk** is ingeschakeld in de kaart **Diensten** onder **Instellingen, Integraties**. Een server in een ander netwerk of achter een VPN verschijnt daar niet.
- Een zelfondertekend certificaat wordt één keer vertrouwd op basis van de SHA-256-vingerafdruk. Toont de server later een ander certificaat, dan waarschuwt de app je en opent hij de server pas nadat je het nieuwe certificaat vertrouwt.

## De telefoon op de Instanties-pagina {#instances}

De telefoon krijgt een eigen kaart op de Instanties-pagina van elke server in de groep, gemarkeerd als Android-app en met de naam die je de telefoon hebt gegeven. Hij heeft geen scorecard omdat hij geen back-ups maakt. **Verwijderen** haalt hem van de pagina en beëindigt meteen zijn sessie, net als het uitzetten van **Deze instantie bevragen** onder **Bewaking** in zijn details. De telefoon kent de woorden nog, dus hij verschijnt weer zodra zijn app opent, maar uitgeschakeld, en hij krijgt geen sessie tot iemand hem aanzet. Verwijder voor een verloren telefoon zijn kaart op elke server, zet **Telefoons kunnen koppelen met de twaalf woorden van de groep** uit onder **Instellingen, Beveiliging**, of laat elke instantie **Groep verlaten** en koppel ze opnieuw met nieuwe woorden.

Een telefoon die voor 10.0.0 is gekoppeld, gaat zijn aanroepen ondertekenen bij het eerste bezoek na de update, en elke server bewaart de eerste sleutel die hij voor de telefoon krijgt. Als iemand anders die de woorden van de groep kent er eerder bij is terwijl de telefoon offline is, wijst die server de telefoon vanaf dat moment af, en kan de app hem niet meer openen via de relay. Los dit op door in een browser in te loggen op die server, op de kaart van de telefoon op de Instanties-pagina **Sleutel vergeten** te kiezen en daarna **Vergeten bevestigen**, en de app opnieuw te openen. De server neemt bij de volgende aanroep de eigen sleutel van de telefoon over. Wordt de telefoon opnieuw afgewezen, dan heeft iemand anders de woorden: laat elke instantie **Groep verlaten** en koppel ze opnieuw met nieuwe woorden, net als bij een verloren telefoon.

## Instellingen {#settings}

Het tandwiel naast het plusteken opent de instellingen van de app:

- de taal, en de naam die de telefoon op de Instanties-pagina toont (leeg betekent het model van de telefoon),
- het uiterlijk, dat de eerste server in de lijst volgt tot je zelf iets instelt, en de animaties, die een eigen instelling hebben,
- een rapport om te kopiëren als je een probleem meldt; het bevat geen adres, naam of frase,
- de kaart Over deze app met het privacybeleid,
- **Alle servers verwijderen**, dat elke server uit de app verwijdert en de groep verlaat. Elke server die de telefoon nog bereikt, haalt hem van zijn Instanties-pagina.

## Downloads en uploads {#files}

Exports, herstelkits, flash-ZIP's en databasedumps komen in de map Downloads van de telefoon terecht, net als vanuit een browser. Een import van instellingen opent de bestandskiezer van de telefoon. Via de relay wordt geen bestand opgeslagen; open daarvoor de server op zijn eigen adres.

## Op een touchscreen {#touch}

Onder een vinger zweeft niets, dus een bedieningselement wordt gedimd zolang je het vasthoudt, en een knop met een merklogo licht op in zijn merkkleur tot je je vinger optilt. Lang drukken op een knop telt als een trage tik en opent geen linkmenu. Een tik op een grijze knop toont waarom hij grijs is, tot je ergens anders tikt.

## In plaats daarvan in een browser {#browser}

Chrome en Edge kunnen de webinterface van BombVault installeren als app in een eigen venster, op een telefoon net als op een computer. Er wordt niets in de cache bewaard, dus een update is meteen zichtbaar.

## Privacy {#privacy}

De app heeft geen accounts, geen advertenties en geen analytics, en hij draait niets op de achtergrond. Het [privacybeleid](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) vermeldt wat de app opslaat en wat hij waarheen stuurt.
