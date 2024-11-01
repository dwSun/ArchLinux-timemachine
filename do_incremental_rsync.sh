#!/bin/bash

# Usage:
#  do_incremental_rsync.sh /some/directory /and/some/other/directory
#  do_incremental_rsync.sh                (will backup /)
#
#  do_incremental_rsync.sh -v /home /     (-v = print verbose rsync output)
#
# rsync uses --one-file-system, so if you have several filesystems under / you need to supply them as separate arguments to the script
#
# More information in the README or http://ekenberg.github.io/linux-timemachine/

function die {
    echo >&2 $@
    exit 1
}

ping -c 1 -w 1 192.168.5.149 &>/dev/null && result=0 || result=1
if [ "$result" == 0 ];then
    echo -e "online"
else
    die -e "\033[31;1m Server is down!\033[0m"
fi

# Go to directory of this script
cd "$( dirname "$0" )"

if [ `id -u` != "0" ]; then
    echo "You need to be root (or sudo) to run system backups"
    exit 1
fi

# check for exclude-file
[ -f backup_exclude.conf ] || die "You must supply backup_exclude.conf (empty is ok)"


TODAY=`date +"%Y-%m-%d"`


function backup {
    
    [ -z "${BACKUP_BASE}" ] && die "BACKUP_BASE not configured in backup.conf"
    [ -z "${BACKUP_NAME}" ] && die "BACKUP_NAME not configured in backup.conf"
    [ -z "${BACKUP_WHAT}" ] && die "BACKUP_WHAT not configured in backup.conf"
    
    echo "Backup [${BACKUP_WHAT}] to: ${BACKUP_BASE}/${BACKUP_NAME}-${TODAY}"
    
    CURRENT_BACKUP="${BACKUP_BASE}/${BACKUP_NAME}-current"
    NEW_BACKUP="${BACKUP_BASE}/${BACKUP_NAME}-${TODAY}"
    
    mkdir -p "$NEW_BACKUP"
    
    [ -d "$NEW_BACKUP" ] || die "No such directory: $NEW_BACKUP"
    
    echo "rsync -avpPe ssh --delete --relative --one-file-system --numeric-ids --exclude-from=backup_exclude.conf --link-dest=\"$CURRENT_BACKUP\" \"$BACKUP_WHAT\" \"$NEW_BACKUP\""
    rsync -avpPe ssh --delete --relative --one-file-system --numeric-ids --exclude-from=backup_exclude.conf --link-dest="$CURRENT_BACKUP" "$BACKUP_WHAT" "$NEW_BACKUP"
    
    # Update soft link to current backup
    [ -h "$CURRENT_BACKUP" ] && rm -f "$CURRENT_BACKUP"
    ln -s "$NEW_BACKUP" "$CURRENT_BACKUP" || die "Cannot create soft link '$NEW_BACKUP' -> '$CURRENT_BACKUP'"
}


for conf in $(ls config); do
    echo "Using config: $conf"
    source config/$conf || die "Cannot read config file: $conf"
    backup&
done

wait
