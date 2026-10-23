# Aplikacja na Androida

Aplikacja na Androida przenosi na telefon wszystkie serwery BombVault z Twojej grupy. Startuje od listy Twoich serwerów, nad którą widać dziennik aktywności ich wszystkich, te same wiersze, które pokazuje przegląd, i otwiera widok telefoniczny serwera, który stukniesz. Sama aplikacja nie tworzy żadnych kopii zapasowych.

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

Aplikacja dołącza do grupy przez przekaźnik, także w domu. Jeśli twoja instancja ma ustawione **Brak relay**, otwórz **Ustawienia, Parowanie** i wybierz przekaźnik, zanim zeskanujesz kod.

## Poza domem {#away}

Aplikacja najpierw próbuje połączyć się z serwerem pod jego własnym adresem. Gdy ten nie odpowiada, na przykład w sieci komórkowej, otwiera interfejs serwera przez przekaźnik grupy. Serwer grupy, którego adresu aplikacja jeszcze nie zna, otwiera się tak samo. Jeśli nie odpowiada ani adres, ani przekaźnik, informuje o tym ekran startowy na karcie serwera.

Przez przekaźnik strona sięga tego, czego sparowany telefon sięga w domu, jak opisano niżej. Logowanie hasłem i zapisywanie plików działają tylko pod własnym adresem serwera. Paski postępu odświeżają się co dwie sekundy, a nie na bieżąco. Pliki interfejsu zostają w telefonie po pierwszej wizycie, więc kolejne wizyty ładują się szybciej. Serwer starszy niż 10.0.0 otwiera się tylko pod własnym adresem.

Aplikacja przyjmuje odpowiedź tylko wtedy, gdy serwer podpisał ją kluczem, który aplikacja przypięła przy pierwszym kontakcie z tym serwerem. Jeśli serwer później odpowie innym kluczem, ekran startowy pokaże odcisk nowego klucza, a strona otworzy się dopiero wtedy, gdy mu zaufasz.

## Przełączanie serwerów {#switch}

Gdy serwer jest otwarty, u góry jego przeglądu widnieje nazwa serwera, a przed nią strzałka powrotu. Strzałka prowadzi do przeglądu aplikacji, czyli ekranu startowego, który wyświetla wszystkie serwery znane aplikacji wraz z ich stanem: **Połączono**, **Brak połączenia**, **Wymagane logowanie** lub **Sprawdź certyfikat**. Serwer z grupy, który nie odpowiada, ma stan **Nieaktywne**, jest szary i pokazuje, kiedy grupa widziała go ostatnio, a serwer, od którego aplikacja jeszcze nic nie usłyszała, pokazuje **Sprawdzanie**. Przy każdym połączeniu aplikacja odczytuje listę grupy z jednej z jej instancji, więc na liście jest też instancja, której nigdy nie spotkała przez przekaźnik, na przykład taka, która była wyłączona, gdy telefon dołączał do grupy. Dotknij tam innego, aby go otworzyć, w domu bezpośrednio, a poza domem przez przekaźnik.

## Logowanie na sparowanym telefonie {#sign-in}

Telefon sparowany z Twoją grupą otwiera każdy z jej serwerów od razu zalogowany. Zanim załaduje stronę, prosi ten serwer przez grupę o sesję, a serwer wydaje ją tylko członkowi, który jest telefonem. Serwer dodany po adresie pyta o hasło, tak jak w przeglądarce.
Sparowany telefon nie dostaje wszystkiego, co daje hasło. Jego sesja sięga do stałej listy tego, co pokazuje aplikacja: przeglądu, uruchomień i dziennika aktywności, kopii zapasowych do podejrzenia, uruchomienia i przywrócenia, anomalii do przeczytania oraz strony Instancje do podejrzenia. Miejsca docelowe i polecenia wracają bez adresów i sekretów. Przywrócenie, które zastępuje oryginał, najpierw tworzy kopię zapasową bieżącego stanu. Klucze, zestaw odzyskiwania, depozyty kluczy, ustawienia i usuwanie kopii zapasowych zostają przy logowaniu na samym serwerze, a wszystko inne odpowiada, że potrzebuje tego logowania. Przez przekaźnik obowiązuje ta sama lista, także na serwerze bez hasła.

Przełącznik tego ustawienia to **Telefony mogą się sparować z dwunastoma słowami grupy** w karcie **Aplikacja na Androida** w **Ustawienia, Bezpieczeństwo**. Zresetowanie hasła wyłącza go i wylogowuje wszystkie telefony, dopóki nie włączysz go tam z powrotem.

## Serwery spoza grupy {#other-servers}

- **Dodaj serwer** przyjmuje adres, pod którym otwierasz BombVault w przeglądarce, na przykład `192.168.1.10:3443`. Bez `http://` lub `https://` na początku aplikacja używa https.
- Serwery, które ogłaszają się w sieci lokalnej, są wymienione w sekcji **W tej sieci** i otwierają się jednym stuknięciem. Ogłaszają się, dopóki w karcie **Usługi** w **Ustawienia, Integracje** włączone jest **Znajdź w sieci**. Serwer w innej sieci albo za VPN się tam nie pojawi.
- Certyfikatowi samopodpisanemu ufa się raz, na podstawie jego odcisku SHA-256. Gdy serwer później pokaże inny, aplikacja Cię ostrzeże i otworzy go dopiero wtedy, gdy zaufasz nowemu certyfikatowi.

## Telefon na stronie Instancje {#instances}

Telefon dostaje własną kartę na stronie Instancje każdego serwera w grupie, oznaczoną jako aplikacja na Androida i z nazwą nadaną telefonowi. Nie ma karty wyników, bo niczego nie kopiuje. **Usuń** zdejmuje go ze strony i od razu kończy jego sesję, podobnie jak wyłączenie **Odpytuj tę instancję** w sekcji **Monitorowanie** w jego szczegółach. Telefon nadal zna słowa, więc pojawia się znowu, gdy otworzy się jego aplikacja, ale wyłączony, i nie dostaje sesji, dopóki ktoś go nie włączy. W razie zgubienia telefonu usuń jego kartę na każdym serwerze, wyłącz **Telefony mogą się sparować z dwunastoma słowami grupy** w **Ustawienia, Bezpieczeństwo** albo na każdej instancji wybierz **Opuść grupę** i sparuj je ponownie z nowymi słowami.

Telefon sparowany przed wersją 10.0.0 zaczyna podpisywać swoje wywołania przy pierwszej wizycie po aktualizacji, a każdy serwer zachowuje pierwszy klucz, jaki dostanie dla telefonu. Jeśli ktoś inny, kto zna słowa grupy, zdąży pierwszy, gdy telefon jest offline, ten serwer od tej pory odrzuca telefon, a aplikacja nie może go już otworzyć przez relay. Aby to naprawić, zaloguj się na ten serwer w przeglądarce, wybierz **Zapomnij klucz** na karcie telefonu na stronie Instancje, potem **Potwierdź zapomnienie** i otwórz aplikację ponownie. Serwer przejmie własny klucz telefonu przy następnym wywołaniu. Jeśli telefon zostanie odrzucony ponownie, słowa ma ktoś inny: na każdej instancji wybierz **Opuść grupę** i sparuj je ponownie z nowymi słowami, tak jak w przypadku zgubionego telefonu.

## Ustawienia {#settings}

Zębatka obok plusa otwiera ustawienia aplikacji:

- język oraz nazwa, pod którą telefon widnieje na stronie Instancje (puste pole oznacza model telefonu),
- wygląd, który podąża za pierwszym serwerem na liście, dopóki nie ustawisz własnego, oraz animacje, które mają osobne ustawienie,
- raport do skopiowania, gdy zgłaszasz problem; nie zawiera adresu, nazwy ani frazy,
- karta O aplikacji z polityką prywatności,
- **Usuń wszystkie serwery**, które usuwa z aplikacji wszystkie serwery i opuszcza grupę. Każdy serwer, do którego telefon jeszcze dociera, usuwa go ze swojej strony Instancje.

## Pobieranie i wysyłanie plików {#files}

Eksporty, zestawy odzyskiwania, pliki ZIP z flasha i zrzuty baz danych trafiają do folderu Pobrane na telefonie, tak jak z przeglądarki. Import ustawień otwiera okno wyboru plików telefonu. Przez przekaźnik żaden plik nie jest zapisywany. Aby to zrobić, otwórz serwer pod jego własnym adresem.

## Na ekranie dotykowym {#touch}

Pod palcem nic nie reaguje na najechanie, więc kontrolka przygasa, dopóki jest przytrzymana, a przycisk z logo marki świeci w kolorze marki, dopóki nie podniesiesz palca. Długie przytrzymanie przycisku liczy się jako powolne stuknięcie i nie otwiera menu linku. Stuknięcie wyszarzonego przycisku pokazuje, dlaczego jest wyszarzony, dopóki nie stukniesz w inne miejsce.

## W przeglądarce zamiast aplikacji {#browser}

Chrome i Edge mogą zainstalować interfejs WWW BombVault jako aplikację we własnym oknie, na telefonie tak samo jak na komputerze. Nic nie jest buforowane, więc aktualizacja jest widoczna od razu.

## Prywatność {#privacy}

Aplikacja nie ma kont, reklam ani analityki i nie uruchamia niczego w tle. Jej [polityka prywatności](https://github.com/junkerderprovinz/bombvault/blob/main/android/PRIVACY.md) wymienia, co przechowuje i co dokąd wysyła.
