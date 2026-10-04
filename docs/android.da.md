# Android-app

Android-appen giver dig alle BombVault-servere i din gruppe på din telefon. Den starter med en liste over dine servere med aktivitetsloggen for dem alle øverst, de samme linjer som oversigten viser, og åbner telefonvisningen for den server, du trykker på. Appen tager ikke selv nogen sikkerhedskopier.

## Hent appen {#install}

- **APK:** hver udgivelse har `bombvault-android.apk` på sin udgivelsesside, og [dette link](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) henter altid det nyeste build. Den kræver Android 10 eller nyere, og Android spørger én gang, om den app, du åbner filen med, må installere apps.
- **Google Play:** appen er i en lukket test, indtil den kan blive offentlig. Google Play viser først en app fra en ny udviklerkonto, når mindst 12 testere har haft den installeret i 14 dage. Vil du hjælpe, så meld dig ind i [testergruppen](https://groups.google.com/g/arrowloop-testers), åbn [testsiden](https://play.google.com/apps/testing/bombvault.halleluja.design), tryk på **Bliv tester**, og installér BombVault fra Google Play.
- **F-Droid:** opføringen kommer senere.

Par appen med servere på version 9.7.0 eller nyere. En server på en ældre version kan være i samme gruppe, men den viser telefonen som en almindelig instans, og appen læser først den servers aktivitet efter et login.

## Parring via QR-kode {#pairing}

1. Åbn **Indstillinger, Parring** på en vilkårlig server i din gruppe, og vælg **Vis sætning**. De tolv ord vises med en QR-kode ved siden af.
2. Tryk på **Scan QR-kode** i appen, og ret telefonen mod koden. Du kan også indsætte eller skrive ordene.
3. Appen viser serverne i gruppen, før den gemmer noget. **Tilføj alle** tilføjer dem allesammen.

Derefter går telefonen med i gruppen som endnu en instans. Den læser via gruppen, hvad der kører på hver server, uden login, direkte når du er hjemme og over relayet, når du er ude. Hvordan selve gruppen fungerer, er beskrevet under [Parring af instanser](offsite-recovery.md#pairing).

!!! note "Grænsefladen skal stadig kunne nå serveren"
    Serverlisten og aktivitetsloggen kommer via gruppen. En servers grænseflade åbnes direkte, så telefonen skal kunne nå serverens adresse, hjemme eller over en VPN.

## Logget ind på en parret telefon {#sign-in}

En telefon, der er parret med din gruppe, åbner hver af gruppens servere allerede logget ind. Før den indlæser en side, beder den serveren om en session via gruppen, og en server udleverer kun en session til et medlem, der er en telefon. En server, der er tilføjet med sin adresse, beder om adgangskoden, ligesom i en browser. Alle, der har de tolv ord, kan allerede åbne alle gruppens sikkerhedskopier, så parring giver ikke adgang til noget nyt.

## Servere uden for en gruppe {#other-servers}

- **Tilføj server** tager den adresse, du åbner BombVault med i en browser, for eksempel `192.168.1.10:3443`. Uden `http://` eller `https://` foran bruger appen https.
- Servere, der annoncerer sig selv på det lokale netværk, vises under **På dette netværk** og åbnes med ét tryk. Det gør de, så længe **Find på netværket** er slået til under Indstillinger, Integrationer. En server på et andet netværk eller bag en VPN dukker ikke op der.
- Et selvsigneret certifikat bliver stolet på én gang ud fra sit SHA-256-fingeraftryk. Viser serveren senere et andet, advarer appen dig og åbner den først, når du stoler på det nye certifikat.

## Telefonen på Instanser-siden {#instances}

Telefonen får sit eget kort på Instanser-siden på hver server i gruppen, markeret som Android-app og med det navn, du har givet telefonen. Den har intet scorekort, fordi den ikke tager sikkerhedskopier, og **Fjern** tager den af siden.

## Indstillinger {#settings}

Tandhjulet ved siden af plusset åbner appens indstillinger:

- sproget og det navn, telefonen viser på Instanser-siden (tomt betyder telefonens model),
- udseendet, der følger den første server på listen, indtil du vælger dit eget, og animationerne, der har deres egen indstilling,
- en rapport til at kopiere, når du melder et problem; den indeholder ingen adresse, intet navn og ingen sætning,
- kortet Om appen med privatlivspolitikken,
- **Fjern alle servere**, der fjerner alle servere fra appen og forlader gruppen. Intet ændres på selve serverne.

## Downloads og uploads {#files}

Eksporter, gendannelseskit, flash-ZIP'er og databasedumps havner i telefonens mappe Downloads, ligesom fra en browser. En import af indstillinger åbner telefonens filvælger.

## På en touchskærm {#touch}

Intet svæver under en finger, så en kontrol dæmpes, mens den holdes nede, og en knap med et brandlogo lyser i sin brandfarve, indtil fingeren slipper. Et langt tryk på en knap tæller som et langsomt tryk og åbner ingen linkmenu.

## I en browser i stedet {#browser}

Chrome og Edge kan installere BombVaults webgrænseflade som en app i sit eget vindue, på en telefon som på en computer. Intet gemmes i cache, så en opdatering vises med det samme.

## Privatliv {#privacy}

Appen har ingen konti, ingen reklamer og ingen analyse, og den kører intet i baggrunden. Dens [privatlivspolitik](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) beskriver, hvad den gemmer, og hvad den sender hvorhen.
