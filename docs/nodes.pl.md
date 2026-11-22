# Węzły

Węzeł to BombVault bez interfejsu WWW, na przykład na VPS-ie albo na małym serwerze u znajomego. Robi kopie zapasowe kontenerów i folderów na własnym serwerze, a BombVault, na którym go dodajesz, na tej stronie nazywany **instancją domową**, mówi mu, kiedy robić kopie i dokąd mają trafiać. Węzłem zarządzasz z interfejsu WWW instancji domowej, obok własnych elementów.

![Karta węzła vps-falkenstein na stronie Instancje: Połączono, jego wersja, jego kopie na serwerze odbiorczym, kiedy każda sekcja robi kopie, oraz Zmień miejsce docelowe, Zastąp i Zwolnij.](assets/screenshots/nodes/card-dark.png#only-dark)
![Karta węzła vps-falkenstein na stronie Instancje: Połączono, jego wersja, jego kopie na serwerze odbiorczym, kiedy każda sekcja robi kopie, oraz Zmień miejsce docelowe, Zastąp i Zwolnij.](assets/screenshots/nodes/card-light.png#only-light)

## Jak to działa {#how-it-works}

- **Instancja domowa trzyma klucz węzła.** Gdy dodajesz węzeł, instancja domowa tworzy jego `APP_KEY`, zachowuje jego zapieczętowaną kopię i przekazuje go Tobie raz, w pliku compose węzła. Dzięki tej kopii instancja domowa dociera do węzła i otwiera jego kopie zapasowe.
- **Węzeł nie potrzebuje otwartego portu.** Instancja domowa i węzeł rozmawiają przez relay własnym, prywatnym łączem, oddzielnie od Twojej grupy parowania. Instancja domowa zaczyna każde wywołanie i je podpisuje, a węzeł nie prosi instancji domowej o nic poza swoim powitaniem. Jedyne miejsce docelowe, które wymaga otwartego portu, to [trzymanie kopii na węźle](#keep-on-node).
- **Kopie zapasowe nigdy nie idą przez relay.** Węzeł wysyła je prosto tam, gdzie wybierzesz: na serwer odbiorczy instancji domowej, do jednego z miejsc docelowych instancji domowej albo na rest-server na samym węźle.
- **Węzeł niczego sam nie planuje.** Nie uruchamia harmonogramów, nie wysyła powiadomień i nie dołącza do żadnej grupy ani twierdzy. Instancja domowa uruchamia jego kopie, pilnuje, jak idą, i wysyła alerty.
- **Węzeł kopiuje kontenery i zestawy folderów.** Maszyny wirtualne, dysk flash, zbiory danych ZFS i autokopia zostają przy pełnych instalacjach.

Łącze używa własnego relay instancji domowej, gdy jej grupa go ma, a w przeciwnym razie relay projektu, także wtedy, gdy instancja domowa paruje się bez relay.

## Zanim zaczniesz {#before-you-start}

- **Hasło logowania na instancji domowej.** Dodanie węzła, ponowne pokazanie jego pliku compose i otwieranie jego elementów wymagają zalogowania hasłem, zobacz [Logowanie](configuration.md#login).
- **Docker z Compose na serwerze węzła.**
- **Miejsce na kopie zapasowe**, jedno z trzech opisanych w [Dokąd trafiają kopie](#placement). Najbezpieczniejszy jest serwer odbiorczy, bo węzeł nie może tam niczego usunąć.

## Dodawanie węzła {#add}

1. Na instancji domowej otwórz **Instancje** i wybierz **Dodaj węzeł** w prawym dolnym rogu.
2. **Nazwa.** Wpisz, jak węzeł ma się tu nazywać, na przykład tak jak serwer, i wybierz **Dalej**.

    ![Okno Dodaj węzeł w pierwszym kroku, z wpisaną nazwą vps-helsinki.](assets/screenshots/nodes/add-name-dark.png#only-dark)
    ![Okno Dodaj węzeł w pierwszym kroku, z wpisaną nazwą vps-helsinki.](assets/screenshots/nodes/add-name-light.png#only-light)

3. **Kopie zapasowe.** Wybierz, dokąd trafiają kopie, i uzupełnij to, o co pyta dane miejsce. **Robi kopie codziennie o** to godzina startu obu sekcji. Instancja domowa proponuje pół godziny po własnych kopiach kontenerów, żeby oba zadania nie sięgały naraz po te same dyski. Później na karcie możesz dać każdej sekcji własny plan. Wybierz **Dalej**.

    ![Drugi krok: wybrany serwer odbiorczy, Adres dla węzłów wypełniony adresem https i Robi kopie codziennie o 03:00.](assets/screenshots/nodes/add-placement-dark.png#only-dark)
    ![Drugi krok: wybrany serwer odbiorczy, Adres dla węzłów wypełniony adresem https i Robi kopie codziennie o 03:00.](assets/screenshots/nodes/add-placement-light.png#only-light)

4. **Uruchomienie.** Okno pokazuje plik compose węzła z wypełnionymi `MODE`, `APP_KEY`, `NODE_HOME` i `NODE_NAME`. Skopiuj go i schowaj `APP_KEY` w bezpiecznym miejscu. Na serwerze węzła:

    ```sh
    mkdir -p /opt/bombvault-node && cd /opt/bombvault-node
    nano docker-compose.yml   # wklej plik z okna
    docker compose up -d
    ```

    Zanim go uruchomisz, skieruj wolumen Host Data na dane, które chcesz kopiować, i ustaw `HOST_SOURCE_ROOT` na tę samą ścieżkę, tak jak w przykładzie poniżej.

5. Okno czeka na węzeł. Gdy węzeł odpowie, instancja domowa mówi mu, dokąd trafiają jego kopie, i sprawdza, czy węzeł tam dociera, a okno pokazuje **Węzeł vps-helsinki jest online i przyjmuje polecenia od tej instancji.**

    ![Ostatni krok: plik compose, a potem węzeł online i jego kopie skierowane na serwer odbiorczy.](assets/screenshots/nodes/add-start-dark.png#only-dark)
    ![Ostatni krok: plik compose, a potem węzeł online i jego kopie skierowane na serwer odbiorczy.](assets/screenshots/nodes/add-start-light.png#only-light)

Okno możesz zamknąć, zanim węzeł odpowie. Jego karta pokazuje wtedy **Oczekiwanie na węzeł**, a instancja domowa powie mu, dokąd trafiają kopie, gdy tylko węzeł się pojawi.

### Przykład: VPS, który robi kopie na serwer odbiorczy {#example-vps}

Instancja domowa to maszyna Unraid o nazwie `tower`, na której działa [serwer odbiorczy](offsite-recovery.md#receiving-server). Reverse proxy publikuje go jako `https://backup.tower.example.org`. Na VPS-ie `vps-helsinki` działają Nextcloud i Caddy, a ich dane leżą w `/srv/appdata`.

1. Na `tower`: **Instancje**, **Dodaj węzeł**, nazwa `vps-helsinki`, **Dalej**.
2. Wybierz **Serwer odbiorczy tej instancji** i wpisz `https://backup.tower.example.org` w **Adres dla węzłów**. Kontenery instancji domowej robią kopie o 02:30, więc okno proponuje 03:00. Zostaw to i wybierz **Dalej**.
3. Na VPS-ie zapisz plik z okna, zmieniając wolumeny i `HOST_SOURCE_ROOT` na `/srv/appdata`:

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

4. `docker compose up -d`. Kilka sekund później okno na `tower` mówi, że węzeł jest online, i **Kopie zapasowe trafiają do miejsca https://backup.tower.example.org.**
5. O 03:00 instancja domowa uruchamia kopie węzła. Następnego ranka karta **Węzły** w przeglądzie pokazuje ostatnią kopię `vps-helsinki`, a **Kopie zapasowe** wymieniają jego kontenery w osobnej sekcji.

Jak BombVault znajduje dane kontenera na zwykłym hoście Docker, opisuje [Wykrywanie źródeł kopii zapasowej](configuration.md#backup-source-detection).

## Dokąd trafiają kopie {#placement}

Każdy węzeł ma jedno miejsce docelowe dla obu swoich sekcji. **Zmień miejsce docelowe** na jego karcie zmienia je, gdy węzeł jest online. Nowe miejsce dostaje nowe repozytoria, a kopie w starym miejscu zostają tam, gdzie są.

### Serwer odbiorczy tej instancji {#to-receiver}

Węzeł zapisuje na serwer odbiorczy instancji domowej w trybie append-only: może dodawać kopie, ale żadnej nie usunie. Dostaje tam własny login, nazwany od węzła, i po jednym repozytorium na sekcję we własnym folderze. Instancja domowa dodaje do każdego repozytorium własny klucz i sama je porządkuje, bo węzeł tego nie potrafi.

Jeśli instancja domowa nie ma jeszcze serwera odbiorczego, krok to mówi i proponuje **Skonfiguruj serwer odbiorczy**. Instancja domowa nie potrzebuje do tego grupy. Wybierz folder na udziale i port, kliknij **Skonfiguruj i dalej**, a potem **Gotowe**, gdy okno pokaże serwer, i krok idzie dalej z nim. Szczegóły są w sekcji [Serwer odbiorczy](offsite-recovery.md#receiving-server).

Węzeł dociera do serwera pod **Adres dla węzłów**, który ustawiasz w drugim kroku albo w oknie **Serwer odbiorczy**. Węzeł zwykle wysyła z internetu, więc adres musi być jednym z tych:

- `https://`, na przykład za reverse proxy, Pangolinem albo tunelem. Gdy serwer pokazuje własny certyfikat zamiast certyfikatu publicznego urzędu, wklej go w formacie PEM w **Certyfikat**.
- `http://` z prywatnym adresem IP, na przykład `10.0.0.5` na łączu WireGuard albo `100.101.102.103` w Tailscale.

Zwykłe `http://` na publiczny adres jest odrzucane, bo węzeł wysyłałby kopie przez internet bez szyfrowania. Gdy zmienisz adres, instancja domowa powiadamia każdy węzeł korzystający z odbiornika, a węzeł, którego akurat nie ma, dowie się o tym po powrocie.

### Miejsce docelowe {#to-location}

Węzeł zapisuje do jednego z miejsc docelowych instancji domowej, do którego sam dociera: do usługi S3 albo serwera REST, przez `https://` albo pod prywatnym adresem. Jego repozytoria leżą tam w `bombvault-node-<node id>-containers` i `bombvault-node-<node id>-files`.

Węzeł loguje się [zapisanymi danymi logowania](offsite-recovery.md#sign-ins) utworzonymi dla niego, które muszą być inne niż te, którymi instancja domowa zapisuje w tym miejscu. Najpierw je dodaj, jeśli okno mówi, że ich nie ma. Z tymi danymi logowania węzeł zwykle może usuwać to, co leży w miejscu docelowym, inaczej niż na serwerze odbiorczym, i okno to mówi.

### Trzymaj je na węźle {#keep-on-node}

Węzeł sam trzyma swoje kopie za własnym rest-serverem w trybie append-only, a instancja domowa pobiera kopię każdej sekcji do własnej pamięci. To pasuje do węzła, któremu instancja domowa nie może zaoferować miejsca. To jedyne miejsce docelowe, które wymaga otwartego portu.

![Drugi krok z wybranym Trzymaj je na węźle, adresem https://203.0.113.7:8443 i uwagą o otwartym porcie.](assets/screenshots/nodes/add-keep-dark.png#only-dark)
![Drugi krok z wybranym Trzymaj je na węźle, adresem https://203.0.113.7:8443 i uwagą o otwartym porcie.](assets/screenshots/nodes/add-keep-light.png#only-light)

1. W **Adres węzła** wpisz, gdzie instancja domowa dociera do węzła: jego publiczny adres i wolny port, na przykład `https://203.0.113.7:8443`, bez folderu na końcu. `http://` używaj tylko z prywatnym adresem, jak wyżej.
2. Węzeł uruchamia na tym porcie kontener `bombvault-node-rest-server`, z repozytoriami w folderze `bombvault-node` pod Host Data. Przy `https://` tworzy certyfikat dla tego adresu, a instancja domowa go przypina.
3. Przepuść port przez zaporę węzła, żeby instancja domowa mogła do niego dotrzeć. Port zajęty przez inny kontener na węźle jest odrzucany z nazwą tego kontenera.

Instancja domowa pobiera sekcję zaraz po zakończeniu jej kopii na węźle, a awaryjnie co godzinę, kwadrans po pełnej, także przy wyłączonym **Pobieranie**. To, co instancja domowa pobrała, zostaje na instancji domowej jak każda [pobrana kopia](offsite-recovery.md#receiver), nawet gdy węzła już nie ma. Węzeł sam porządkuje swoje repozytoria, gdy instancja domowa o to poprosi, bo jego rest-server nie przyjmuje usunięć, a jego zestawy folderów pomijają folder `bombvault-node`, więc nigdy nie kopiuje własnych kopii.

## Kiedy węzły robią kopie {#schedule}

Harmonogramem każdego węzła zarządza instancja domowa. Karta pokazuje plan dla każdej sekcji, na przykład **Containers** *codziennie o 2:00* i **Foldery** *co tydzień (nd) o 4:00*. Kliknięcie otwiera ten sam edytor co w [Harmonogramy](schedules.md#plans), a zmiana zapisuje się od razu.

![Plan kontenerów węzła otwarty z jego karty, z Wył., Codziennie, Co tydzień, Co N dni i Cron.](assets/screenshots/nodes/schedule-dark.png#only-dark)
![Plan kontenerów węzła otwarty z jego karty, z Wył., Codziennie, Co tydzień, Co N dni i Cron.](assets/screenshots/nodes/schedule-light.png#only-light)

- **Węzeł, którego nie było**, uruchamia pominięte kopie, gdy tylko wróci. Instancja domowa sprawdza co pięć minut.
- **Zajęty węzeł** dostaje ponowne zapytanie pięć minut później.
- **Gdy instancja domowa porządkuje** stare kopie węzła, węzeł niczego nie uruchamia, dopóki instancja domowa nie skończy. Na serwerze odbiorczym i na samym węźle to okno otwiera się w każdą niedzielę o 03:00 i trwa do sześciu godzin. W miejscu docelowym idzie za planem kopii poza siedzibą, jaki instancja domowa ma dla tego miejsca. Instancja domowa przechowuje kopie według własnej reguły sekcji w **Jak długo przechowywać** albo według reguły miejsca, jeśli ją ma.
- **Na liście przebiegów węzła** kopie i przywracania uruchomione przez instancję domową są oznaczone jako uruchomione przez instancję domową.

## Pilnowanie węzłów {#watch}

Karta **Węzły** w przeglądzie wymienia każdy węzeł z jego stanem, ostatnią kopią, błędami i otwartymi anomaliami. Pojawia się, gdy masz jakiś węzeł.

![Karta Węzły w przeglądzie z trzema węzłami: jeden połączony, jeden nieaktywny z dwoma błędami i jeden wciąż oczekujący na węzeł.](assets/screenshots/nodes/overview-card-dark.png#only-dark)
![Karta Węzły w przeglądzie z trzema węzłami: jeden połączony, jeden nieaktywny z dwoma błędami i jeden wciąż oczekujący na węzeł.](assets/screenshots/nodes/overview-card-light.png#only-light)

Instancja domowa wysyła jeden alert na epizod przez swoje [kanały powiadomień](notifications.md), a następny dopiero wtedy, gdy ten epizod się skończy:

- **Kopia się nie udała**, na przykład *BombVault: vps-helsinki: backup failed*, z elementem i przyczyną. Epizod kończy późniejsza udana kopia tego elementu.
- **Węzeł jest poza zasięgiem**, na przykład *BombVault: vps-helsinki is out of reach*. Instancja domowa czeka tyle, ile wynosi najkrótszy wyłącznik martwego człowieka na repozytoriach węzła, czyli 26 godzin, jeśli tego nie zmienisz, albo 6 godzin, dopóki węzeł nie ma jeszcze miejsca docelowego.

Nadzór odbiornika na instancji domowej obejmuje też każde repozytorium węzła, z wyłącznikiem martwego człowieka i codziennym `restic check` o 04:00, zobacz [Odbiornik i pobierający](offsite-recovery.md#receiver).

## Elementy węzła {#entries}

W **Kopie zapasowe** listy Containers i Foldery mają pod Twoimi elementami sekcję dla każdego węzła. Własny kontener węzła jest pominięty. **Utwórz kopię teraz** w menu elementu uruchamia jego kopię na węźle, a **Otwórz** otwiera jego stronę.

W sekcji węzła na liście Foldery **Dodaj zestaw folderów** dodaje zestaw folderów na węźle, a **Edytuj** w menu zestawu go zmienia. Oba otwierają to samo okno co dla Twoich własnych zestawów. Wybór folderów w tym oknie przegląda foldery węzła, ale nie tworzy tam nowych.

![Sekcja węzła vps-falkenstein na liście Containers: nextcloud, nextcloud-db, caddy i uptime-kuma.](assets/screenshots/nodes/backups-section-dark.png#only-dark)
![Sekcja węzła vps-falkenstein na liście Containers: nextcloud, nextcloud-db, caddy i uptime-kuma.](assets/screenshots/nodes/backups-section-light.png#only-light)

Strona elementu węzła pokazuje **Na węźle** i nazwę węzła oraz wymienia jego kopie. **Przywróć** przy jednej z nich przywraca element na węźle i zastępuje to, co jest tam teraz. Kontener zostaje na ten czas zatrzymany, tak jak na instancji domowej.

![Strona nextcloud na vps-falkenstein z jego kopiami i przyciskiem Przywróć przy każdej.](assets/screenshots/nodes/entry-dark.png#only-dark)
![Strona nextcloud na vps-falkenstein z jego kopiami i przyciskiem Przywróć przy każdej.](assets/screenshots/nodes/entry-light.png#only-light)

Przez instancję domową docierasz do stałej listy własnych tras węzła: jego kontenerów, zestawów folderów, przebiegów i anomalii oraz do robienia ich kopii i przywracania. Ustawienia, klucze i pamięć węzła pozostają poza zasięgiem, a nic na tej liście niczego nie usuwa. Zobacz [Węzły w API](api.md#nodes).

## Zastępowanie i zwalnianie {#replace-release}

**Zastąp** pokazuje ponownie plik compose węzła, po zalogowaniu hasłem. Użyj go, żeby przenieść węzeł na inny serwer albo go odbudować: uruchom tam plik i wyłącz stary serwer. Ten sam `APP_KEY` i `NODE_HOME` sprawiają, że to znów ten sam węzeł, z jego kopiami i historią. Instancja domowa przechowuje dla każdego węzła jego zestawy folderów, kontenery z tym, czego trzeba, żeby je odtworzyć, i przebiegi. Gdy węzeł startuje z pustym folderem `config`, instancja domowa przyjmuje go ponownie, mówi mu, dokąd idą jego kopie, i przed następną kopią oddaje mu to wszystko, więc węzeł może przywrócić kontener, którego już nie ma. Na nowym serwerze węzeł, który sam trzyma swoje kopie, zaczyna z pustymi repozytoriami. Jego wcześniejsze kopie zostają w tych, które pobrała instancja domowa. Dwa serwery z tym samym `APP_KEY` liczą się jako jeden węzeł, więc nigdy nie uruchamiaj obu.

**Zwolnij** puszcza węzeł wolno. Przestaje przyjmować polecenia od instancji domowej i nie robi już żadnych kopii. Jego kopie zostają tam, gdzie są, podobnie jak każda kopia, którą pobrała instancja domowa. Jego login na serwerze odbiorczym znika, a instancja domowa przestaje pilnować jego repozytoriów. Węzeł, którego nie ma, gdy go zwalniasz, zachowuje kartę ze stanem **Trwa zwalnianie**, dopóki się o tym nie dowie. Zwolniony węzeł nie przyjmie już żadnej instancji domowej z tym samym `APP_KEY`. Żeby znów użyć tego serwera, dodaj go jako nowy węzeł i uruchom z nowym plikiem compose i pustym folderem `config`.

## Bezpieczeństwo {#security}

Jeśli instancja domowa zostanie przejęta, atakujący kontroluje też jej węzły, a węzeł ma w ręku Dockera swojego serwera, czyli tam roota. Chroń instancję domową hasłem i trzymaj ją z dala od internetu. Jeśli przejęty zostanie węzeł, atakujący nie dostaje grupy parowania, twierdzy, innego węzła ani innego repozytorium i nie może usunąć historii na serwerze odbiorczym. Szczegóły są w [Węzły w modelu bezpieczeństwa](configuration.md#nodes).

[Zestaw odzyskiwania](offsite-recovery.md#recovery-kit) ma rozdział **Węzły** z `APP_KEY` każdego węzła, jego hasłem restic i jego repozytoriami, więc możesz przywrócić węzeł albo jego kopie zwykłym resticiem.
