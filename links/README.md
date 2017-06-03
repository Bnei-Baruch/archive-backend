# Links backend

## Overview

Some basic reasoning for this service:

The requirements for public, external, links to archive files are:

1. All links are pointing to this service.

2. This service dictates the name of the file the user see.
This allows the physical storage systems to ignore file names
and the application to change a file's name at will.

3. This service communicates with the filer-backend to retrieve a url
for accessing the physical file based on geo location.
 Once we get a url we redirect the client to that url.


### High Availability & Disaster Recovery

**All links** from all sites (not only the main archive site) will link to this service.
Thus, we must be online at all times and have a DNS change based DR plan.


### Point of failure

The MDB read replica should be the only PoF for this service


For all of the above reasons we need this service to be as thin as possible
and the deployment should be separated from the archive site backend.
