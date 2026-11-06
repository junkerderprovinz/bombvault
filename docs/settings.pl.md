# Ustawienia

**Ustawienia** to czternaście kafelków. Każdy kafelek to strona z kartami, a każda zmiana jest zapisywana na bieżąco, z krótkim błyskiem na kontrolce albo drżeniem, gdy serwer ją odrzuci. Kilka okien zachowuje przycisk **Zapisz**, bo zapisanie ich w połowie byłoby niebezpieczne, jak logowanie albo hasło logowania. Wyjaśnienia stoją w dymkach (i) obok każdej kontrolki.

Naciśnij `/` albo Ctrl+K gdziekolwiek w ustawieniach, aby przeszukać je wszystkie, także po nazwach, jakie strony miały w dawniejszych wersjach. Kolejność kafelków można zmienić w **Wygląd**. Gdy kolumna kafelków obok strony by ją ścisnęła, jak w oknie o szerokości około 1200 pikseli albo w telefonie, kafelki zmieniają się w pasek nad stroną, który przewija się na boki.

![Ustawienia z otwartym kafelkiem Ogólne.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Ustawienia z otwartym kafelkiem Ogólne.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Ta strona wymienia każdy kafelek i jego karty oraz odsyła do strony, która objaśnia każdą z nich dokładniej.

## Ogólne {#general}

- **Sekcje** włączają lub wyłączają każdą sekcję: **Containers**, **Maszyny wirtualne**, **Flash**, **Foldery**, **Zbiory danych ZFS** i **Autokopia**. Sekcja, która jest wyłączona, nie jest kopiowana według żadnego harmonogramu, także według własnego planu elementu, a jej elementy się nie liczą. **Utwórz kopię teraz** nadal działa. **Flash** widać tylko na Unraid.
- **Containers** zawierają to, co dotyczy każdego kontenera, jak **Automatyczne zrzuty baz danych**. Wszystko, co dotyczy jednego kontenera, jest na jego stronie. Zobacz [Zrzuty baz danych](backups.md#database-dumps).

## Wygląd {#look}

- **Wygląd**: **Język** (42 języki), **Motyw** (**Ciemny**, **Jasny** lub **Systemowy**), **Narożniki**, **Animacja** (**Wył.**, **Subtelny**, **Dziki** lub **Burzliwy**), **Ciche powiadomienia**, które ukrywają komunikaty o sukcesie, jak potwierdzenia zapisu, oraz **Kolejność stron ustawień**, gdzie przytrzymujesz kafelek i przeciągasz go na miejsce albo używasz Alt i klawiszy strzałek.
- **Etykiety**: ile z kontrolki jest pokazane, osobno dla **Przyciski**, **Panel boczny**, **Karty** i **Panel dolny**: **Tekst**, **Tekst i symbol**, **Symbol** albo **Reaktywny**, który pokazuje sam symbol i wsuwa słowa, gdy go wskażesz. **Panel boczny** dotyczy tylko układu na komputerze, a **Panel dolny** tylko układu w telefonie.
- **Kolory**: jeden **Kolor akcentu** albo **Tryb tęczowy**, w którym każdy wiersz listy dostaje własny kolor z palety ośmiu. Tryb tęczowy dodaje **Tryb disco**, w którym kolory powoli przechodzą do następnego, **Tryb reaktywny**, w którym kolor wiersza widać tylko, gdy go wskażesz, gdy działa albo jest zaznaczony, **Rotacja kolorów** i samą **Paletę kolorów**.

BombVault trzyma wygląd na serwerze. Przeglądarka, która przychodzi pierwszy raz, przejmuje go. Gdy Twój system prosi o mniej ruchu, to zawsze wygrywa z **Animacja**.

## Harmonogram {#schedule}

**Tydzień**, **Pełna kopia** z **Kiedy działają kopie**, **Nadrabiaj pominięte kopie zapasowe po starcie** i **Polecenia przed i po**, **Harmonogramy według sekcji** z **Kopie poza siedzibą według sekcji**, **Zaplanowane elementy**, **Kolejność** z **Ponowne uruchomienie po kopii zapasowej**, **Migawka zamiast zatrzymania**, **Aktualizacje po kopii** i **Bezczynność przed kopią**. Zobacz [Harmonogramy](schedules.md).

## Miejsca docelowe {#storage-locations}

- **Pokaż także na pasku bocznym** przypina stronę Miejsca docelowe do paska bocznego.
- Lista Twoich miejsc docelowych z **Dodaj miejsce docelowe**. Kliknięcie otwiera stronę miejsca, zobacz [Strona miejsca docelowego](offsite-recovery.md#location-page).
- **Dane logowania**: zapisane logowania z gwiazdką domyślną, zobacz [Dane logowania](offsite-recovery.md#sign-ins).
- **Miejsca docelowe według sekcji**: przepływ 3-2-1, gdzie każda linia jest przełącznikiem, zobacz [Ustawienia domyślne dla sekcji](offsite-recovery.md#placement-defaults).
- **Repozytoria**: repozytoria utworzone dla pojedynczych elementów, zobacz [Ustawienie dla elementu](offsite-recovery.md#placement).
- **Jak długo przechowywać** wskazuje miejsca docelowe, gdzie ustawia się przechowywanie, a **Podgląd** pokazuje **Co usunęłoby następne uruchomienie**, dla każdego repozytorium i elementu, zanim to nastąpi. Niczego nie zmienia i niczego nie blokuje, więc odpowiada nawet podczas kopii.
- **Kopie poza siedzibą**: jeden wiersz na sekcję z **Testuj połączenie**, **Replikuj teraz** i **Skonfiguruj** albo **Edytuj**, zobacz [Kopie poza siedzibą](offsite-recovery.md#off-site-copies).
- **Przepustowość poza siedzibą**: **Limit wysyłania (KiB/s)** i **Limit pobierania (KiB/s)** dla każdej kopii. 0 znaczy bez limitu.

Karta **Streaming**, która obniża limit wysyłania, gdy Plex, Jellyfin lub Emby streamują, leży na samej stronie Miejsca docelowe.

## Przywracanie {#restore}

- Link do strony **Przywracanie**.
- **Foldery przywracania**: **Domyślny folder przywracania** dla przywracania do folderu, z **Podfolder dla każdej sekcji** i **Wyjątek** dla każdej sekcji. Zobacz [Dokąd trafia przywracanie do folderu](restore.md#folders).
- **Rejestry kontenerów**: logowania do prywatnych rejestrów, jak obraz `ghcr.io` dla sponsorów. Przywracanie używa ich do pobrania brakującego obrazu, a aktualizacja po kopii do sprawdzenia nowszego. Obrazy publiczne są pobierane bez nich. **Dodaj rejestr** oferuje rejestry publiczne, własne, te u dostawcy chmury i **Inny rejestr** dla dowolnego adresu.
- **Na wypadek awarii**: **Pobierz zestaw odzyskiwania**, który wymaga hasła logowania. Zobacz [Zestaw odzyskiwania](offsite-recovery.md#recovery-kit).

## Powiadomienia {#notifications}

**Powiadomienia** z **Powiadom**, podsumowaniami, **Zgłaszaj opóźnione kopie zapasowe**, **Powiadomienia Unraid** i **Cotygodniowe podsumowanie**, **Kanały** z Webhook, Apprise, Matrix, Gotify i E-mail (SMTP) oraz **Healthchecks**. Zobacz [Powiadomienia](notifications.md).

## Weryfikacja {#verify}

- **Sprawdź kopie zapasowe**: wynik ostatniego `restic check` dla każdej sekcji, z **Odblokuj** i **Przytnij** pod **Więcej** w każdym wierszu. **Odblokuj** czyści starą blokadę, a **Przytnij** stosuje reguły przechowywania na żądanie.
- **Automatyczne kontrole**: zaplanowane sprawdzanie przywracania z **Próbka danych (%)** i celami testów, **Test uruchomienia**, który przywraca też jeden kontener do odizolowanej kopii i ją uruchamia, **Zaplanowana próba DR poza siedzibą** i **Sprawdzaj ochronę przed usunięciem**, czyli zaplanowany test manipulacji. Zobacz [Próby DR](offsite-recovery.md#dr-drills) i [Test manipulacji](offsite-recovery.md#tamper-test).
- **Anomalie**: wykrywanie, czułość i to, które znaleziska wysyłają wiadomość. Zobacz [Anomalie](anomalies.md).

## Bezpieczeństwo {#security}

- **Hasło**: hasło logowania, co najmniej 12 znaków. Zmiana lub wyczyszczenie prosi o obecne i, przy włączonym uwierzytelnianiu dwuskładnikowym, o kod.
- **Uwierzytelnianie dwuskładnikowe**: kod z aplikacji uwierzytelniającej dodatkowo do hasła, z ośmioma kodami odzyskiwania pokazanymi raz.
- **Klucze dostępu**: logowanie kluczem w telefonie, laptopie albo kluczu sprzętowym. Potrzebują prawdziwej nazwy domeny i certyfikatu, któremu ufa przeglądarka.
- **Aplikacja na Androida**: **Telefony mogą się sparować z dwunastoma słowami grupy**, co pozwala sparowanemu telefonowi zalogować się bez hasła.
- **Szyfrowanie**: **Szyfruj kopie zapasowe**, domyślnie włączone.

Kilka rzeczy wymaga hasła logowania, jak zestaw odzyskiwania i depozyty kluczy. Zobacz [Model bezpieczeństwa](configuration.md#security-model).

## Instancje {#instances}

- **Pokaż także na pasku bocznym** przypina stronę Instancje do paska bocznego.
- **Wiele serwerów** włącza części grupy: **Instancje**, stronę z kartą dla każdego członka, **Odbiornik** i **Pobieranie**. Zobacz [Instancje i role](offsite-recovery.md#roles).
- Link do strony Instancje, z częściami, które są włączone.

## Parowanie {#pairing}

**Parowanie** z **Wygeneruj frazę** albo **Wpisz frazę**, karty kroków, nazwa tej instancji i **Znane adresy**, karta **Relay** z jej czterema formami i **Co widzi Relay**, członkowie grupy oraz **Serwer ZFS bez BombVault** jako cel repliki przez SSH. Zobacz [Parowanie instancji](offsite-recovery.md#pairing) i [Wysyłanie na serwer ZFS](zfs-datasets.md#replica-push).

## Integracje {#integrations}

- **Serwer MCP**: klucze dla asystentów AI, **Wybierz klienta** i dziennik każdego klucza. Zobacz [Serwer MCP](mcp.md).
- **Tokeny API**: nazwane tokeny dla skryptów i pulpitów. Zobacz [Tokeny](api.md#tokens).
- **Usługi**: **Home Assistant** przez MQTT, **Znajdź w sieci** (mDNS jako `bombvault.local`) i **Udostępnij /metrics** dla Prometheusa, z **Token scrapowania (opcjonalnie)**. Zobacz [Home Assistant](api.md#home-assistant) i [Znajdź BombVault w sieci](api.md#mdns).
- **Widżet pulpitu**: token tylko do odczytu i adres dla dowolnego pulpitu pokazującego iframe, jak Homepage czy Heimdall, z samym dziennikiem aktywności na żywo. Zobacz [Widżet pulpitu](api.md#dashboard-widget).
- **SSH hosta**: klucz publiczny BombVault i **Testuj połączenie**, dla kopii maszyn wirtualnych i zbiorów danych ZFS oraz powiadomień Unraid. Zobacz [przewodnik po kopiach maszyn wirtualnych](vm-backup-ssh-setup.md).

## Aplikacje {#apps}

- **Aplikacja na Androida**: APK dla wydania, na którym działa Twój serwer, z kodem QR i wpisem F-Droid. Zobacz [Aplikacja na Androida](android.md#install).
- **ParleyPort**: kontener relay z szablonem Unraid i poleceniem Docker, które go uruchamia.
- **BombVault Widget**: kafelek na Pulpicie Unraid z dziennikiem aktywności BombVault i następnym przebiegiem. Przy połączeniu SSH z hostem karta sama instaluje i usuwa wtyczkę. Bez niego podaje adres `.plg` do zainstalowania w **Plugins, Install Plugin**.

## System {#system}

- **Eksport / import ustawień**: cała Twoja konfiguracja jako przenośny plik JSON, z danymi logowania albo bez nich. Import najpierw pokazuje **Ten plik zawiera** i pomija linki do miejsc docelowych, których już nie ma. **Szyfruj eksporty za pomocą age** zabezpiecza zwykłe eksporty, ZIP flash i zestaw odzyskiwania dla wymienionych **Odbiorcy age**. Zobacz [Przenośne ustawienia](configuration.md#portable-settings-export-and-import).
- **Wydajność**: **Wątki CPU (0 = wszystkie)**, **Limit rozmiaru pamięci podręcznej (MB, 0 = bez limitu)** dla pamięci podręcznej restic w `/config` i **Limit pamięci na kopię zapasową (MiB, 0 = brak)**, miękki limit, który spowalnia dużą kopię zamiast zabijać ją za brak pamięci.

## Info {#info}

- **Wersja**: BombVault, GlimStone, restic i rclone, z **Sprawdź aktualizacje** i **Nowości**, które liczą zmiany, jakich jeszcze nie widziałeś, i otwierają okno, które je wymienia.
- **Pomoc**: **Podręcznik** otwiera tę dokumentację, a **Zgłoszenie błędu** pobiera ZIP z usuniętymi danymi wrażliwymi do zgłoszenia na GitHubie. Wymaga hasła logowania.
- **O programie BombVault**: projekt, jego licencja i jak skontaktować się z autorem.
