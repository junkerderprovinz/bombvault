# Android-app

Android-appen gir deg alle BombVault-servere i gruppen din på telefonen. Den starter med en liste over serverne dine med aktivitetsloggen for alle øverst, de samme linjene som oversikten viser, og åpner telefonvisningen for serveren du trykker på. Appen tar ingen sikkerhetskopier selv.

## Skaff deg appen {#install}

- **Innstillinger, Apper:** kortet for Android-appen tilbyr APK-en for utgivelsen serveren din kjører, med en QR-kode du kan skanne fra telefonen.
- **APK:** hver utgivelse har `bombvault-android.apk` på utgivelsessiden, og [denne lenken](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) laster alltid ned det nyeste bygget. Den krever Android 10 eller nyere, og Android spør én gang om appen du åpner filen med, kan installere apper.
- **Google Play:** appen er i en lukket test til den kan bli offentlig. Google Play viser en app fra en ny utviklerkonto først når minst 12 testere har hatt den installert i 14 dager. Vil du hjelpe til, bli med i [testergruppen](https://groups.google.com/g/arrowloop-testers), åpne [testsiden](https://play.google.com/apps/testing/bombvault.halleluja.design), trykk på **Bli tester** og installer BombVault fra Google Play.
- **F-Droid:** oppføringen kommer senere.

Par appen med servere på versjon 9.7.0 eller nyere. En server på en eldre versjon kan være i samme gruppe, men den viser telefonen som en vanlig instans, og appen leser aktiviteten til den serveren først etter en pålogging.

## Paring med QR-kode {#pairing}

1. Åpne **Innstillinger, Paring** på en hvilken som helst server i gruppen din, og velg **Vis frase**. De tolv ordene vises med en QR-kode ved siden av.
2. Trykk på **Skann QR-kode** i appen, og hold telefonen mot koden. Du kan også lime inn eller taste inn ordene.
3. Appen viser serverne i gruppen før den lagrer noe. **Legg til alle** legger til alle sammen.

Deretter blir telefonen med i gruppen som enda en instans. Den leser via gruppen hva som kjører på hver server, uten pålogging, direkte hjemme og over relayet når du er borte. Hvordan selve gruppen fungerer, står under [Koble instanser sammen](offsite-recovery.md#pairing).

Appen blir med i gruppen via relayet, også hjemme. Står instansen din på **Ingen relay**, velg en under **Innstillinger, Paring** før du skanner koden.

## Borte fra hjemmet {#away}

Appen prøver først en server på dens egen adresse. Svarer den ikke, for eksempel på mobilnettet, åpner appen i stedet grensesnittet til serveren via relayet til gruppen, og en linje under siden sier **Åpnet via relay**. En server i gruppen som appen ikke kjenner adressen til ennå, åpnes på samme måte. Svarer verken adressen eller relayet, står det på kortet til serveren på startskjermen.

Via relayet når siden det en paret telefon når hjemme, som beskrevet nedenfor. Pålogging med passordet og lagring av filer fungerer bare på serverens egen adresse. Fremdriftslinjer oppdateres hvert andre sekund i stedet for fortløpende. Filene til grensesnittet blir liggende på telefonen etter det første besøket, så senere besøk laster raskere. En server eldre enn 10.0.0 åpnes bare på sin egen adresse.

## Pålogget på en paret telefon {#sign-in}

En telefon som er paret med gruppen din, åpner hver server i gruppen allerede pålogget. Før den laster en side, ber den serveren om en økt via gruppen, og en server gir bare ut en økt til et medlem som er en telefon. En server som er lagt til med adressen sin, spør etter passordet, slik som i en nettleser.
En paret telefon får ikke alt et passord gir. Økten når en fast liste over det appen viser: oversikten, kjøringene og aktivitetsloggen, sikkerhetskopiene man kan se på, starte og gjenopprette, avvikene man kan lese og Instanser-siden man kan se på. Mål og kommandoer kommer tilbake uten adresser og hemmeligheter. En gjenoppretting som erstatter originalen, tar først en sikkerhetskopi av dagens tilstand. Nøkler, gjenopprettingssettet, nøkkeldeponeringer, innstillinger og sletting av sikkerhetskopier hører til en pålogging på selve serveren, og alt annet svarer at det trenger den påloggingen. Via relayet gjelder den samme listen, også på en server uten passord.

Bryteren for dette er **Telefoner kan pare med gruppens tolv ord** i kortet **Android-app** under **Innstillinger, Sikkerhet**. En tilbakestilling av passordet slår den av og logger alle telefoner ut, til du slår den på igjen der.

## Servere utenfor en gruppe {#other-servers}

- **Legg til server** tar adressen du åpner BombVault med i en nettleser, for eksempel `192.168.1.10:3443`. Uten `http://` eller `https://` foran bruker appen https.
- Servere som annonserer seg selv på det lokale nettverket, vises under **På dette nettverket** og åpnes med ett trykk. Det gjør de så lenge **Finn på nettverket** er slått på i kortet **Tjenester** under **Innstillinger, Integrasjoner**. En server i et annet nettverk eller bak en VPN dukker ikke opp der.
- Et selvsignert sertifikat blir klarert én gang ut fra SHA-256-fingeravtrykket. Viser serveren senere et annet, advarer appen deg og åpner serveren først når du stoler på det nye sertifikatet.

## Telefonen på Instanser-siden {#instances}

Telefonen får sitt eget kort på Instanser-siden på hver server i gruppen, merket som Android-app og med navnet du har gitt telefonen. Den har ikke noe poengkort, fordi den ikke tar sikkerhetskopier, og **Fjern** tar den bort fra siden.

## Innstillinger {#settings}

Tannhjulet ved siden av plusstegnet åpner innstillingene i appen:

- språket og navnet telefonen viser på Instanser-siden (tomt betyr telefonens modell),
- utseendet, som følger den første serveren i listen til du velger ditt eget, og animasjonene, som har en egen innstilling,
- en rapport å kopiere når du melder fra om et problem; den inneholder ingen adresse, ikke noe navn og ingen frase,
- kortet Om appen med personvernerklæringen,
- **Fjern alle servere**, som fjerner alle servere fra appen og forlater gruppen. Ingenting endres på selve serverne.

## Nedlastinger og opplastinger {#files}

Eksporter, gjenopprettingssett, flash-ZIP-filer og databasedumper havner i mappen Nedlastinger på telefonen, akkurat som fra en nettleser. En import av innstillinger åpner filvelgeren på telefonen. Via relayet lagres ingen fil. Åpne serveren på sin egen adresse for det.

## På en berøringsskjerm {#touch}

Ingenting svever under en finger, så en kontroll dempes mens den holdes nede, og en knapp med en merkelogo lyser i merkefargen sin til fingeren slipper. Et langt trykk på en knapp teller som et sakte trykk og åpner ingen lenkemeny.

## I en nettleser i stedet {#browser}

Chrome og Edge kan installere webgrensesnittet til BombVault som en app i et eget vindu, på en telefon som på en datamaskin. Ingenting bufres, så en oppdatering vises med en gang.

## Personvern {#privacy}

Appen har ingen kontoer, ingen reklame og ingen analyse, og den kjører ingenting i bakgrunnen. [Personvernerklæringen](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) viser hva den lagrer og hva den sender hvor.
