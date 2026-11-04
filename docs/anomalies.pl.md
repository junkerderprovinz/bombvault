# Anomalie

Ransomware i zepsuta aplikacja wyglądają w kopii zapasowej podobnie: nagle większość danych jest nowa albo większości brakuje. BombVault porównuje każdą kopię z własną historią danego elementu i zgłasza znalezisko, gdy coś wygląda źle. Dopóki otwarte jest znalezisko o utraconych danych, zachowuje też stare kopie tego elementu, żeby ostatnia dobra nie zdążyła się zestarzeć. Wpis **Anomalie** na pasku bocznym jest widoczny, gdy wykrywanie jest włączone.

![Strona Anomalie z jednym otwartym znaleziskiem krytycznym.](assets/screenshots/anomalies/page-dark.png#only-dark)
![Strona Anomalie z jednym otwartym znaleziskiem krytycznym.](assets/screenshots/anomalies/page-light.png#only-light)

## Strona {#page}

Kafelki **Krytyczna**, **Ostrzeżenia** i **Uwagi** wybierają wagę, a **Otwarte** i **Zamknięte** nad listą przełączają między tym, co jest otwarte, i tym, co zamknięto w ostatnich 30 dniach. Znalezisko krytyczne leży na tej samej powierzchni co pozostałe, a jego waga widać po plakietce i kolorowej krawędzi. Znalezisko podsuwa najpierw sposób naprawy, a w szczegółach pokazuje wartość jako krzywą na tle zwykłego zakresu.

Gdy nic nie jest otwarte, **Postęp nauki** pokazuje dla każdego elementu pierścień z dziesięciu części, po jednej na każdą kopię, z której BombVault się uczył. **Monitorowanie** otwiera listę elementów z ich własnymi ustawieniami.

## Co jest sprawdzane {#checks}

Każda kopia kontenera, maszyny wirtualnej, zestawu folderów, zrzutu bazy danych, dysku flash i autokopii jest porównywana z historią swojego elementu. Kontrole dotyczą:

- nowych danych zapisanych w przebiegu, w zestawieniu z największymi zwykłymi ilościami z ostatnich kopii oraz ze zwykłym tempem na godzinę,
- kopii, która zapisała ponownie większość danych, w tym pliki zmieniające nazwę i zapisane od nowa,
- rozmiaru źródła i liczby plików, które restic podaje dla każdego elementu i każdego zrzutu bazy danych,
- własnego czasu kopii według restic,
- serii błędów i błędów występujących co jakiś czas,
- sprawdzeń przywracania, które przestały przechodzić,
- wolnego miejsca w repozytoriach lokalnych, SFTP i rclone, prognozowanego na podstawie przyrostu repozytorium.

Element uczy się na pierwszych 10 kopiach. Prawie puste źródło, nadpisanie większości danych i błędy są sprawdzane od samego początku.

Znalezisko o rozmiarze lub liczbie plików źródła jest datowane na pierwszą kopię, w której się pojawiło. **Porównaj z poprzednią kopią** wymienia foldery, z których pliki zniknęły, doszły albo się zmieniły, z uwagą, gdy prawie wszystko leży w indeksie wyszukiwania, pamięci podręcznej lub miniaturach, które aplikacja odtwarza sama.

Elementy ZFS są obserwowane zbiór danych po zbiorze, zobacz [Zbiory danych ZFS](zfs-datasets.md#anomalies). Asystent może czytać otwarte znaleziska przez [serwer MCP](mcp.md#tools).

Kontrole mają granice. Repozytoria S3, B2 i REST nie podają wolnego miejsca, na udziale użytkownika Unraid wolne miejsce dotyczy całej macierzy, a kopie zrobione przed restic 0.17 nie mają historii rozmiaru.

## Zamykanie znaleziska {#closing}

- **Ostrzeżenia** zamykają się same, gdy przyczyna ustąpi.
- **Krytyczne** znaleziska o utraconych danych i zapełniającym się dysku zostają, dopóki czegoś nie zrobisz.
- **Potwierdź** oznacza znalezisko jako zobaczone i zamyka je. Nie zostanie zgłoszone ponownie, dopóki jego przyczyna raz nie zniknie.
- **Oznacz jako oczekiwane** mówi, że zmiana była Twoja. Nowy poziom staje się normalny po 10 kopiach, ale kontrola prawie pustego źródła nigdy nie jest wyłączana.
- Zmieniony wybór, na przykład zaznaczony lub odznaczony folder, sam zaczyna historię elementu od nowa.

Powiadomienie wychodzi raz na jeden epizod, a błędy i sprawdzenia przywracania, które już powiadamiają, nie są zgłaszane dwa razy.

## Stare kopie są w tym czasie zachowywane {#retention-hold}

Gdy źródło jest prawie puste, mocno się skurczyło albo większość jego danych zapisano ponownie, przechowywanie i przycinanie omijają stare kopie tego elementu, a znalezisko wskazuje ostatnią dobrą kopię, którą okno przywracania zaznacza z góry. Zanim zamkniesz takie znalezisko, BombVault powie, że następne czyszczenie znów usunie stare kopie według reguł przechowywania. Jeśli więc brakuje danych albo zostały zaszyfrowane, najpierw przywróć je, a znalezisko zamknij dopiero potem.

## Ustawienia {#settings}

Karta **Anomalie** w **Ustawienia, Weryfikacja** zawiera ustawienia globalne. Każdy przełącznik zapisuje się od razu po zmianie, a trzy pod przełącznikiem głównym są ukryte, gdy wykrywanie jest wyłączone.

| Ustawienie | Domyślnie | Co robi |
|---|---|---|
| **Wykrywaj anomalie** | Włączone | Porównuje każdą kopię z historią jej elementu. Po wyłączeniu nic nowego nie jest sprawdzane, a wpis **Anomalie** znika z paska bocznego. Karta nadal prowadzi do dawnych znalezisk. |
| **Czułość** | Zrównoważona | Surowa zgłasza mniejsze zmiany, Łagodna tylko duże. |
| **Wysyłaj powiadomienie dla** | Tylko krytyczne znaleziska | Najniższa waga, która wysyła wiadomość kanałami ustawionymi w [Powiadomieniach](notifications.md). Powtarzające się błędy kopii i zrzutów oraz nieudane zaplanowane sprawdzenia przywracania wysyłają już własną wiadomość i nie są wysyłane dwa razy. |
| **Zachowuj stare kopie, gdy źródło mocno się kurczy lub zostaje nadpisane** | Włączone | Dopóki element ma otwarte znalezisko o prawie pustym źródle, gwałtownym skurczeniu albo ponownie zapisanej większości danych, przechowywanie i przycinanie omijają jego stare kopie. Potwierdź znalezisko albo oznacz je jako oczekiwane, żeby je puścić. |

Każdy element może mieć własną czułość i własne minimum powiadomień. Ustawisz je na karcie **Weryfikacja** elementu albo na stronie **Anomalie** w sekcji **Monitorowanie**. Dla elementu ZFS obowiązują dla każdego zbioru danych jego drzewa.
