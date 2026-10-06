# Aplikacja na Androida

Aplikacja na Androida przenosi na telefon wszystkie serwery BombVault z Twojej grupy. Startuje od listy Twoich serwerów, nad którą widać dziennik aktywności ich wszystkich, te same wiersze, które pokazuje Panel, i otwiera widok telefoniczny serwera, który stukniesz. Sama aplikacja nie tworzy żadnych kopii zapasowych.

## Instalacja aplikacji {#install}

- **Ustawienia, Aplikacje:** karta aplikacji na Androida oferuje plik APK dla wydania, które uruchamia Twój serwer, wraz z kodem QR do zeskanowania z telefonu.
- **APK:** każde wydanie ma na swojej stronie plik `bombvault-android.apk`, a [ten link](https://github.com/junkerderprovinz/bombvault/releases/latest/download/bombvault-android.apk) zawsze pobiera najnowszą kompilację. Aplikacja wymaga Androida 10 lub nowszego, a Android raz zapyta, czy aplikacja, w której otwierasz plik, może instalować aplikacje.
- **Google Play:** aplikacja jest w teście zamkniętym, dopóki nie będzie mogła stać się publiczna. Google Play udostępnia aplikację z nowego konta dewelopera dopiero wtedy, gdy co najmniej 12 testerów miało ją zainstalowaną przez 14 dni. Jeśli chcesz pomóc, dołącz do [grupy testerów](https://groups.google.com/g/arrowloop-testers), otwórz [stronę testu](https://play.google.com/apps/testing/bombvault.halleluja.design), stuknij **Zostań testerem** i zainstaluj BombVault z Google Play.
- **F-Droid:** wpis pojawi się później.

Paruj aplikację z serwerami w wersji 9.7.0 lub nowszej. Serwer w starszej wersji może być w tej samej grupie, ale pokazuje telefon jako zwykłą instancję, a aplikacja odczytuje aktywność tego serwera dopiero po zalogowaniu.

## Parowanie kodem QR {#pairing}

1. Na dowolnym serwerze swojej grupy otwórz **Ustawienia, Parowanie** i wybierz **Pokaż frazę**. Pojawi się dwanaście słów, a obok nich kod QR.
2. W aplikacji stuknij **Zeskanuj kod QR** i skieruj telefon na kod. Możesz też wkleić albo wpisać słowa.
3. Zanim aplikacja cokolwiek zapisze, wyświetla listę serwerów tej grupy. **Dodaj wszystkie** dodaje każdy z nich.

Telefon dołącza wtedy do grupy jak kolejna instancja. To, co działa na każdym serwerze, odczytuje przez grupę bez logowania, w domu bezpośrednio, a poza domem przez przekaźnik. Jak działa sama grupa, opisuje [Parowanie instancji](offsite-recovery.md#pairing).

!!! note "Interfejs nadal potrzebuje drogi do serwera"
    Lista serwerów i dziennik aktywności przychodzą przez grupę. Interfejs serwera otwiera się jednak bezpośrednio, więc telefon musi dotrzeć do adresu serwera, w domu albo przez VPN.

## Logowanie na sparowanym telefonie {#sign-in}

Telefon sparowany z Twoją grupą otwiera każdy z jej serwerów od razu zalogowany. Zanim załaduje stronę, prosi ten serwer przez grupę o sesję, a serwer wydaje ją tylko członkowi, który jest telefonem. Serwer dodany po adresie pyta o hasło, tak jak w przeglądarce. Kto zna dwanaście słów, i tak może otworzyć każdą kopię zapasową grupy, więc parowanie nie daje nic nowego.

## Serwery spoza grupy {#other-servers}

- **Dodaj serwer** przyjmuje adres, pod którym otwierasz BombVault w przeglądarce, na przykład `192.168.1.10:3443`. Bez `http://` lub `https://` na początku aplikacja używa https.
- Serwery, które ogłaszają się w sieci lokalnej, są wymienione w sekcji **W tej sieci** i otwierają się jednym stuknięciem. Ogłaszają się, dopóki w Ustawienia, Integracje włączone jest **Znajdź w sieci**. Serwer w innej sieci albo za VPN się tam nie pojawi.
- Certyfikatowi samopodpisanemu ufa się raz, na podstawie jego odcisku SHA-256. Gdy serwer później pokaże inny, aplikacja Cię ostrzeże i otworzy go dopiero wtedy, gdy zaufasz nowemu certyfikatowi.

## Telefon na stronie Instancje {#instances}

Telefon dostaje własną kartę na stronie Instancje każdego serwera w grupie, oznaczoną jako aplikacja na Androida i nazwaną tak, jak nazwałeś telefon. Nie ma karty wyników, bo niczego nie kopiuje, a **Usuń** zdejmuje go ze strony.

## Ustawienia {#settings}

Zębatka obok plusa otwiera ustawienia aplikacji:

- język oraz nazwa, pod którą telefon widnieje na stronie Instancje (puste pole oznacza model telefonu),
- wygląd, który podąża za pierwszym serwerem na liście, dopóki nie ustawisz własnego, oraz animacje, które mają osobne ustawienie,
- raport do skopiowania, gdy zgłaszasz problem; nie zawiera adresu, nazwy ani frazy,
- karta O aplikacji z polityką prywatności,
- **Usuń wszystkie serwery**, które usuwa z aplikacji wszystkie serwery i opuszcza grupę. Na samych serwerach nic się nie zmienia.

## Pobieranie i wysyłanie plików {#files}

Eksporty, zestawy odzyskiwania, pliki ZIP z flasha i zrzuty baz danych trafiają do folderu Pobrane na telefonie, tak jak z przeglądarki. Import ustawień otwiera okno wyboru plików telefonu.

## Na ekranie dotykowym {#touch}

Pod palcem nic nie reaguje na najechanie, więc kontrolka przygasa, dopóki jest przytrzymana, a przycisk z logo marki świeci w kolorze marki, dopóki nie podniesiesz palca. Długie przytrzymanie przycisku liczy się jako powolne stuknięcie i nie otwiera menu linku.

## W przeglądarce zamiast aplikacji {#browser}

Chrome i Edge mogą zainstalować interfejs WWW BombVault jako aplikację we własnym oknie, na telefonie tak samo jak na komputerze. Nic nie jest buforowane, więc aktualizacja jest widoczna od razu.

## Prywatność {#privacy}

Aplikacja nie ma kont, reklam ani analityki i nie uruchamia niczego w tle. Jej [polityka prywatności](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) wymienia, co przechowuje i co dokąd wysyła.
