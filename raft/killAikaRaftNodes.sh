#!/bin/bash

# Filen der vi lagret host:port par
HOST_FILE="data/activehostport.txt"

if [ ! -f "$HOST_FILE" ]; then
    echo "Fant ikke $HOST_FILE. Er nodene startet?"
    exit 1
fi

# Hent ut unike vertsnavn fra filen (kolonne 1 før kolonet)
HOSTS=$(cut -d':' -f1 "$HOST_FILE" | sort -u)

echo "Starter opprydding på følgende noder..."

for HOST in $HOSTS; do
    echo "Sletter prosesser på $HOST..."
    # -f gjør at pkill leter i hele kommandolinjen, ikke bare prosessnavnet
    ssh -n "$USER@$HOST" "pkill -9 -f inf3203_startupRaftNodes" 2>/dev/null
    
    if [ $? -eq 0 ]; then
        echo "  [OK] Prosesser drept på $HOST"
    else
        echo "  [INFO] Ingen aktive prosesser funnet på $HOST"
    fi
done

echo "Opprydding ferdig."