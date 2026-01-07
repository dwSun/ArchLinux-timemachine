#!/bin/bash

# Usage:
#   ./do_incremental_rsync.sh
#
# Description:
#   This script does incremental backups using rsync.
#   It reads configuration files from the config/ directory.
#   Each configuration file must define the following variables:
#     BACKUP_BASE:  Base directory for backups
#     BACKUP_NAME:  Name of the backup
#     BACKUP_HOST:  Hostname of the server to backup
#     BACKUP_WHAT:  What to backup (rsync source)
#     BACKUP_EXCLUDE:  Exclude file for rsync
#

# Exit on error
function die {
    echo >&2 $@
    exit 1
}


# Go to directory of this script
cd "$( dirname "$0" )"

# Check if we are root
if [ `id -u` != "0" ]; then
    echo "You need to be root (or sudo) to run system backups"
    exit 1
fi


TODAY=`date +"%Y-%m-%d-%H"`

function backup {
    [ -z "${BACKUP_BASE}" ] && die "BACKUP_BASE not configured"
    [ -z "${BACKUP_NAME}" ] && die "BACKUP_NAME not configured"
    [ -z "${BACKUP_HOST}" ] && die "BACKUP_HOST not configured"
    [ -z "${BACKUP_WHAT}" ] && die "BACKUP_WHAT not configured"
    [ -z "${BACKUP_EXCLUDE}" ] && die "BACKUP_EXCLUDE not configured"
    
    echo "Backup [${BACKUP_WHAT}] to: ${BACKUP_BASE}/${BACKUP_NAME}-${TODAY}"
    
    # check for exclude-file
    [ -f ./config/exclude/$BACKUP_EXCLUDE ] || die "You must supply backup_exclude.conf (empty is ok)"
    
    # check if server is online
    ping -c 1 -w 1 $BACKUP_HOST &>/dev/null && result=0 || result=1
    if [ "$result" == 0 ];then
        echo -e "online"
    else
        die -e "\033[31;1m Server is down!\033[0m"
    fi
    
    CURRENT_BACKUP="${BACKUP_BASE}/${BACKUP_NAME}-current"
    NEW_BACKUP="${BACKUP_BASE}/${BACKUP_NAME}-${TODAY}"
    
    # Create new backup directory
    mkdir -p "$NEW_BACKUP"
    [ -d "$NEW_BACKUP" ] || die "No such directory: $NEW_BACKUP"
    
    # Do the backup
    echo "rsync -avpPe ssh --delete --relative --one-file-system --numeric-ids --exclude-from=./config/exclude/$BACKUP_EXCLUDE --link-dest=\"$CURRENT_BACKUP\" \"$BACKUP_HOST:$BACKUP_WHAT\" \"$NEW_BACKUP\""
    rsync -avpPe ssh --delete --relative --one-file-system --numeric-ids --exclude-from=./config/exclude/$BACKUP_EXCLUDE --link-dest="$CURRENT_BACKUP" "$BACKUP_HOST:$BACKUP_WHAT" "$NEW_BACKUP"
    
    # Update soft link to current backup
    [ -h "$CURRENT_BACKUP" ] && rm -f "$CURRENT_BACKUP"
    ln -s "$NEW_BACKUP" "$CURRENT_BACKUP" || die "Cannot create soft link '$NEW_BACKUP' -> '$CURRENT_BACKUP'"
}


# Read all config files in config/ and start backup in background
for conf in $(ls config); do
    echo "Using config: $conf"
    source config/$conf || die "Cannot read config file: $conf"
    backup&
done

wait
