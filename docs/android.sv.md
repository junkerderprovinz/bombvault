# Android-app

Android-appen ger dig alla BombVault-servrar i din grupp i telefonen. Den startar med en lista över dina servrar med aktivitetsloggen för dem alla överst, samma rader som översikten visar, och öppnar telefonvyn för den server du trycker på. Appen säkerhetskopierar ingenting själv.

## Skaffa appen {#install}

- **APK:** varje release har `bombvault-android.apk` på sin releasesida, och [den här länken](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) laddar alltid ner det senaste bygget. Den kräver Android 10 eller senare, och Android frågar en gång om appen du öppnar filen med får installera appar.
- **Google Play:** appen är i ett slutet test tills den kan bli offentlig. Google Play listar en app från ett nytt utvecklarkonto först när minst 12 testare har haft den installerad i 14 dagar. Om du vill hjälpa till, gå med i [testargruppen](https://groups.google.com/g/arrowloop-testers), öppna [testsidan](https://play.google.com/apps/testing/bombvault.halleluja.design), tryck på **Bli testare** och installera BombVault från Google Play.
- **F-Droid:** listningen kommer senare.

Parkoppla appen med servrar på version 9.7.0 eller senare. En server på en äldre version kan finnas i samma grupp, men den visar telefonen som en vanlig instans, och appen läser den serverns aktivitet först efter en inloggning.

## Parkoppling med QR-kod {#pairing}

1. Öppna **Inställningar, Parkoppling** på valfri server i din grupp och välj **Visa fras**. De tolv orden visas med en QR-kod bredvid.
2. Tryck på **Skanna QR-kod** i appen och rikta telefonen mot koden. Du kan också klistra in eller skriva orden.
3. Appen listar servrarna i gruppen innan den sparar något. **Lägg till alla** lägger till alla.

Telefonen går sedan med i gruppen som ännu en instans. Den läser via gruppen vad som körs på varje server utan inloggning, direkt hemma och via reläet borta. Hur själva gruppen fungerar beskrivs under [Parkoppling av instanser](offsite-recovery.md#pairing).

!!! note "Gränssnittet behöver fortfarande en väg till servern"
    Serverlistan och aktivitetsloggen kommer via gruppen. En servers gränssnitt öppnas direkt, så telefonen måste nå serverns adress, hemma eller via ett VPN.

## Inloggad på en parkopplad telefon {#sign-in}

En telefon som är parkopplad med din grupp öppnar varje server i gruppen redan inloggad. Innan den laddar en sida ber den servern om en session via gruppen, och en server lämnar bara ut en till en medlem som är en telefon. En server som har lagts till med sin adress frågar efter lösenordet, precis som i en webbläsare. Den som har de tolv orden kan redan öppna varje säkerhetskopia i gruppen, så parkopplingen ger inget nytt.

## Servrar utanför en grupp {#other-servers}

- **Lägg till server** tar adressen du öppnar BombVault med i en webbläsare, till exempel `192.168.1.10:3443`. Utan `http://` eller `https://` framför använder appen https.
- Servrar som annonserar sig i det lokala nätverket listas under **I det här nätverket** och öppnas med ett tryck. Det gör de så länge **Hitta i nätverket** är påslaget under Inställningar, Integrationer. En server i ett annat nätverk eller bakom ett VPN syns inte där.
- Ett självsignerat certifikat betros en gång utifrån sitt SHA-256-fingeravtryck. Om servern senare visar ett annat varnar appen dig och öppnar servern först när du litar på det nya certifikatet.

## Telefonen på Instanser-sidan {#instances}

Telefonen får ett eget kort på Instanser-sidan på varje server i gruppen, markerat som Android-app och med det namn du har gett telefonen. Den har inget poängkort eftersom den inte säkerhetskopierar något, och **Ta bort** tar bort den från sidan.

## Inställningar {#settings}

Kugghjulet bredvid plustecknet öppnar appens inställningar:

- språket och namnet som telefonen visar på Instanser-sidan (tomt betyder telefonens modell),
- utseendet, som följer den första servern i listan tills du ställer in ett eget, och animationerna, som har en egen inställning,
- en rapport att kopiera när du rapporterar ett problem; den innehåller ingen adress, inget namn och ingen fras,
- kortet Om appen med integritetspolicyn,
- **Ta bort alla servrar**, som tar bort alla servrar från appen och lämnar gruppen. Ingenting ändras på själva servrarna.

## Nedladdningar och uppladdningar {#files}

Exporter, återställningskit, flash-ZIP-filer och databasdumpar hamnar i telefonens mapp Nedladdningar, precis som från en webbläsare. En import av inställningar öppnar telefonens filväljare.

## På en pekskärm {#touch}

Ingenting svävar under ett finger, så en kontroll tonas ner medan den hålls nere, och en knapp med en varumärkeslogotyp lyser i sin varumärkesfärg tills fingret lyfts. En lång tryckning på en knapp räknas som ett långsamt tryck och öppnar ingen länkmeny.

## I en webbläsare i stället {#browser}

Chrome och Edge kan installera BombVaults webbgränssnitt som en app i ett eget fönster, i en telefon precis som på en dator. Ingenting cachas, så en uppdatering syns direkt.

## Integritet {#privacy}

Appen har inga konton, ingen reklam och ingen analys, och den kör ingenting i bakgrunden. Dess [integritetspolicy](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) listar vad den sparar och vad den skickar vart.
