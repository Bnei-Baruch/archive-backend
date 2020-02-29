#!/usr/bin/env python


import SimpleHTTPServer
import SocketServer
import atexit
import io
import json
import multiprocessing
import os
import re
import signal
import subprocess
import threading
import urlparse
import sys


PORT = 8000
NUM_WORKER_THREADS = 4
NAME_RE = '[A-Za-z0-9_]+'
LOG_RE = '[A-Za-z0-9_]+.log'

# Demo states.
STARTING = 1
ERROR = 2
STARTED = 3
DOWN = 4

# Demo global table.
demos = {}

def demos_to_json():
    ret = {}
    for name, demo in demos.items():
        d = {}
        for k, v in demo.items():
            if k == 'backend_reindex':
                d[k] = 'Reindexing, please wait...' if v.poll() is None else 'Return code: %d (0 is done and ok)' % v.poll()
            elif k == 'backend_update_synonyms':
                d[k] = 'Updating synonyms, please wait...' if v.poll() is None else 'Return code: %d (0 is done and ok)' % v.poll()
            elif k == 'backend_reindex_grammars':
                d[k] = 'Reindexing grammars, please wait...' if v.poll() is None else 'Return code: %d (0 is done and ok)' % v.poll()
            elif k == 'backend_process':
                d[k] = 'Backend Running' if v.poll() is None else 'Return code: %d (any value means backend is down!)' % v.poll()
            elif k == 'frontend_process':
                d[k] = 'Frontend Running' if v.poll() is None else 'Return code: %d (any value means frontend is down!)' % v.poll()
            #elif k == 'ssr_frontend_process':
            #    d[k] = 'Frontend Running' if v.poll() is None else 'Return code: %d (any value means frontend is down!)' % v.poll()
            else:
                d[k] = v
        ret[name] = d
    return ret

# Ports management handling.
BACKEND_PORTS_START = 9700
backend_ports = {}
FRONTEND_PORTS_START = 4500
frontend_ports = {}

def get_backend_port():
    return get_port(backend_ports, BACKEND_PORTS_START)

def get_frontend_port():
    return get_port(frontend_ports, FRONTEND_PORTS_START)

def free_backend_port(port):
    free_port(backend_ports, port)

def free_frontend_port(port):
    free_port(frontend_ports, port)

def get_port(ports, start):
    port = start
    while port in ports:
        port += 1
    ports[port] = True
    return port

def free_port(ports, port):
    if port in ports:
        del ports[port]

# Run shell command, return return code, stdout and stderr.
def run_command(command, cwd=None, shell=False):
    command_str = command
    if type(command) is list:
        command_str = ' '.join(command)
    print 'running command: [%s]' % command_str
    process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE, cwd=cwd, shell=shell)
    stdout, stderr = process.communicate()
    if shell:
        print 'before wait!'
        returncode = process.wait()
        print 'after wait!'
        return (returncode, stdout, stderr)
    return (process.returncode, stdout, stderr)

def backend_dir(name):
    return '../archive-backend-%s' % name

def frontend_dir(name):
    return '../kmedia-mdb-%s' % name

# Start async tasks.
def start_backend(name):
    demos[name]['backend_process'] = subprocess.Popen(
        './archive-backend server >& ./server.log',
        cwd=backend_dir(name),
        shell=True,
        preexec_fn=os.setsid)
    print 'Started backend_process, pid: %d' % demos[name]['backend_process'].pid
    
def kill_backend(name):
    kill_process(name, 'backend_process')

def start_reindex(name):
    demos[name]['backend_reindex'] = subprocess.Popen(
        './archive-backend index --index_date=%s --update_alias=false >& ./index.log' % name,
        cwd=backend_dir(name),
        shell=True)
    print 'Started backend_reindex, pid: %d' % demos[name]['backend_reindex'].pid
    demos[name]['backend_reindex'].communicate()

def start_reindex_grammars(name):
    demos[name]['backend_reindex_grammars'] = subprocess.Popen(
        './archive-backend index_grammars --index_date=%s --update_alias=false >& ./grammar_index.log' % name,
        cwd=backend_dir(name),
        shell=True)
    print 'Started backend_reindex_grammars, pid: %d' % demos[name]['backend_reindex_grammars'].pid
    demos[name]['backend_reindex_grammars'].communicate()

def start_update_synonyms(name):
    demos[name]['backend_update_synonyms'] = subprocess.Popen(
        './archive-backend update_synonyms --index_date=%s >& ./update_synonyms.log' % name,
        cwd=backend_dir(name),
        shell=True)
    print 'Started backend_update_synonyms, pid: %d' % demos[name]['backend_update_synonyms'].pid
    demos[name]['backend_update_synonyms'].communicate()

def delete_indexes(name):
    (returncode, stdout, stderr) = run_command('./archive-backend delete_index --index_date=%s' % name, backend_dir(name), True)
    if returncode != 0:
        print 'Failed deleting index stderr: %s, stdout: %s returncode: %d' % (stderr, stdout, returncode)
        return (returncode, stderr, stdout)
    else:
        print 'Deleted index %s' % name
    (returncode, stdout, stderr) = run_command('./archive-backend delete_grammar_index --index_date=%s' % name, backend_dir(name), True)
    if returncode != 0:
        print 'Failed deleting grammar index stderr: %s, stdout: %s returncode: %d' % (stderr, stdout, returncode)
        return (returncode, stderr, stdout)
    else:
        print 'Deleted grammar index %s' % name
    return (0, "", "")

def start_frontend(name):
    demos[name]['frontend_process'] = subprocess.Popen(
        'SERVER_PORT=%d NODE_ENV=production node server/index.js >& ./frontend.log' % demos[name]['frontend_port'],
        #'CRA_CLIENT_PORT=%d SERVER_PORT=%d yarn start-server >& ./frontend.log' % (demos[name]['ssr_frontend_port'], demos[name]['frontend_port']),
        cwd=frontend_dir(name),
        shell=True,
        preexec_fn=os.setsid)
    print 'Started frontend_process, pid: %d' % demos[name]['frontend_process'].pid
    #demos[name]['ssr_frontend_process'] = subprocess.Popen(
    #    'PORT=%d yarn start-js >& ./ssr_frontend.log' % (demos[name]['ssr_frontend_port']),
    #    cwd=frontend_dir(name),
    #    shell=True,
    #    preexec_fn=os.setsid)
    #print 'Started ssr_frontend_process, pid: %d' % demos[name]['ssr_frontend_process'].pid

def kill_process(name, process):
    try:
        os.killpg(os.getpgid(demos[name][process].pid), signal.SIGTERM)
        returncode = demos[name][process].wait()
        print 'Killed %s: %d' % (process, returncode)
    except OSError as e:
        print 'failed stopping %s %d: %s' % (process, demos[name][process].pid, e)
    del demos[name][process]
    
def kill_frontend(name):
    kill_process(name, 'frontend_process')
    #kill_process(name, 'ssr_frontend_process')

# Cleanup:
#    1) All background processes stopped automatically. Backend, reindex, grammar reindex, frontend.
#    2) Delete Elasitic indexes.
#    3) Delete directories.
# Clean all running subprocesses on exit.
def cleanup():
    # Processes are killd automatically as they are sub processes of current (shell=True).
    # Delete Elastic indexes.
    for name, demo in demos.items():
        stop_and_clean(name)

def stop_and_clean(name):
    demo = demos[name]
    if demo['elastic'] == 'reindex':
        delete_indexes(name)
    if 'frontend_port' in demo:
        kill_frontend(name)
        free_frontend_port(demo['frontend_port'])
        del demo['frontend_port']
    #if 'ssr_frontend_port' in demo:
    #    free_frontend_port(demo['ssr_frontend_port'])
    #    del demo['ssr_frontend_port']
    if 'backend_port' in demo:
        kill_backend(name)
        free_backend_port(demo['backend_port'])
        del demo['backend_port']
    (returncode, stdout, stderr) = run_command(['rm', '-rf', backend_dir(name)])
    if returncode != 0:
        return 'stderr: %s, stdout: %s' % (stderr, stdout)
    (returncode, stdout, stderr) = run_command(['rm', '-rf', frontend_dir(name)])
    if returncode != 0:
        return 'stderr: %s, stdout: %s' % (stderr, stdout)
    del demos[name]
    print 'stopped and cleaned demo %s' % name

# Register exit cleanup function.
atexit.register(cleanup)
    
def set_up_frontend(name, branch):
    (returncode, stdout, stderr) = run_command(['ls', frontend_dir(name)])
    if returncode == 0:
        return 'Cannot use [%s], already used. stderr: %s, stdout: %s' % (name, stdout, stderr)
    (returncode, stdout, stderr) = run_command(['git', 'clone', 'https://github.com/Bnei-Baruch/kmedia-mdb.git', frontend_dir(name)])
    if returncode != 0:
        return 'stderr: %s, stdout: %s' % (stderr, stdout)
    (returncode, stdout, stderr) = run_command(['git', 'checkout', branch], cwd=frontend_dir(name))
    if returncode != 0:
        return 'stderr: %s, stdout: %s' % (stderr, stdout)
    (returncode, stdout, stderr) = run_command(['cp', '../kmedia-mdb/.env', '%s/.env.demo' % frontend_dir(name)])
    if returncode != 0:
        return 'stderr: %s, stdout: %s' % (stderr, stdout)
    demos[name]['frontend_port'] = get_frontend_port()
    #demos[name]['ssr_frontend_port'] = get_frontend_port()
    (returncode, stdout, stderr) = run_command([
        'sed', '-i', '-E',
        's/REACT_APP_BASE_URL=.*/REACT_APP_BASE_URL=http:\/\/bbdev6.kbb1.com:%d\//g' % demos[name]['frontend_port'],
        '%s/.env.demo' % frontend_dir(name)])
    if returncode != 0:
        return 'stderr: %s, stdout: %s' % (stderr, stdout)
    (returncode, stdout, stderr) = run_command([
        'sed', '-i', '-E',
        's/REACT_APP_API_BACKEND=.*/REACT_APP_API_BACKEND=http:\/\/bbdev6.kbb1.com:%d\//g' % demos[name]['backend_port'],
        '%s/.env.demo' % frontend_dir(name)])
    if returncode != 0:
        return 'stderr: %s, stdout: %s' % (stderr, stdout)
    (returncode, stdout, stderr) = run_command([
        'sed', '-i',
        's/\'default-src\': \[/\'default-src\': [ \'bbdev6.kbb1.com:%d\',/g' % demos[name]['backend_port'],
        '%s/server/app-prod.js' % frontend_dir(name)])
    if returncode != 0:
        return 'stderr: %s, stdout: %s' % (stderr, stdout)
    (returncode, stdout, stderr) = run_command(['yarn', 'install'], cwd=frontend_dir(name))
    if returncode != 0:
        return 'stderr: %s, stdout: %s' % (stderr, stdout)
    (returncode, stdout, stderr) = run_command(['REACT_APP_ENV=demo yarn build >& ./build.log'], cwd=frontend_dir(name), shell=True)
    if returncode != 0:
        return 'stderr: %s, stdout: %s' % (stderr, stdout)
    start_frontend(name)

backend_lock = threading.Lock()
def update_reload(name):
    if demos[name]['elastic'] == 'reindex':
        with backend_lock:
            branch = 'origin/%s' % demos[name]['backend_branch'] 
            (returncode, stdout, stderr) = run_command(['git', 'status'])
            if returncode != 0:
                return 'stderr: %s, stdout: %s' % (stderr, stdout)
            m = re.search(r'# On branch (.*)', stdout)
            if not m:
                m = re.search(r'# HEAD detached at (.*)', stdout)
                if not m:
                    return 'Failed extracting git current branch.'
            original_branch = m.groups(1)[0]
            (returncode, stdout, stderr) = run_command(['git', 'fetch'])
            if returncode != 0:
                return 'stderr: %s, stdout: %s' % (stderr, stdout)
            (returncode, stdout, stderr) = run_command(['git', 'checkout', branch])
            if returncode != 0:
                return 'stderr: %s, stdout: %s' % (stderr, stdout)
            (returncode, stdout, stderr) = run_command(['cp', '-rf', './search/variables', '%s/search/' % backend_dir(name)])
            if returncode != 0:
                return 'stderr: %s, stdout: %s' % (stderr, stdout)
            (returncode, stdout, stderr) = run_command(['cp', '-rf', './search/grammars', '%s/search/' % backend_dir(name)])
            if returncode != 0:
                return 'stderr: %s, stdout: %s' % (stderr, stdout)
            (returncode, stdout, stderr) = run_command(['cp', '-rf', './search/data', '%s/search/' % backend_dir(name)])
            if returncode != 0:
                return 'stderr: %s, stdout: %s' % (stderr, stdout)
            (returncode, stdout, stderr) = run_command(['cp', '-rf', './es/synonyms', '%s/es/' % backend_dir(name)])
            if returncode != 0:
                return 'stderr: %s, stdout: %s' % (stderr, stdout)
            kill_backend(name)
            start_backend(name)
            error = start_update_synonyms(name)
            if error:
                demos[name]['status'].append(error)
            if original_branch != branch:
                (returncode, stdout, stderr) = run_command(['git', 'checkout', original_branch])
                if returncode != 0:
                    return 'stderr: %s, stdout: %s' % (stderr, stdout)
            demos[name]['status'].append('Updated variables, grammars and synonyms. Reloaded backend.')

def set_up_backend(name):
    with backend_lock:
        (returncode, stdout, stderr) = run_command(['ls', backend_dir(name)])
        if returncode == 0:
            return 'Cannot use [%s], already used. stderr: %s, stdout: %s' % (name, stdout, stderr)
        (returncode, stdout, stderr) = run_command(['git', 'status'])
        if returncode != 0:
            return 'stderr: %s, stdout: %s' % (stderr, stdout)
        m = re.search(r'# On branch (.*)', stdout)
        if not m:
            return 'Failed extracting git current branch.'
        original_branch = m.groups(1)[0]
        (returncode, stdout, stderr) = run_command(['git', 'fetch'])
        if returncode != 0:
            return 'stderr: %s, stdout: %s' % (stderr, stdout)
        branch = 'origin/%s' % demos[name]['backend_branch']
        if original_branch != branch:
            (returncode, stdout, stderr) = run_command(['git', 'checkout', branch])
            if returncode != 0:
                return 'stderr: %s, stdout: %s' % (stderr, stdout)
        (returncode, stdout, stderr) = run_command(['make', 'build'])
        if returncode != 0:
            return 'stderr: %s, stdout: %s' % (stderr, stdout)
        else:
            demos[name]['status'].append('Backend binary built')
        (returncode, stdout, stderr) = run_command(['mkdir', backend_dir(name)])
        if returncode != 0:
            return 'stderr: %s, stdout: %s' % (stderr, stdout)
        (returncode, stdout, stderr) = run_command(['cp', './archive-backend', backend_dir(name)])
        if returncode != 0:
            return 'stderr: %s, stdout: %s' % (stderr, stdout)
        (returncode, stdout, stderr) = run_command(['cp', './config.toml', backend_dir(name)])
        if returncode != 0:
            return 'stderr: %s, stdout: %s' % (stderr, stdout)
        (returncode, stdout, stderr) = run_command(['mkdir', '%s/search' % backend_dir(name)])
        if returncode != 0:
            return 'stderr: %s, stdout: %s' % (stderr, stdout)
        (returncode, stdout, stderr) = run_command(['mkdir', '%s/es' % backend_dir(name)])
        if returncode != 0:
            return 'stderr: %s, stdout: %s' % (stderr, stdout)
        (returncode, stdout, stderr) = run_command(['cp', './search/eval.html', '%s/search' % backend_dir(name)])
        if returncode != 0:
            return 'stderr: %s, stdout: %s' % (stderr, stdout)
        demos[name]['backend_port'] = get_backend_port()
        (returncode, stdout, stderr) = run_command([
            'sed', '-i', '-E',
            's/bind-address=\":[0-9]+\"/bind-address=\":%d\"/g' % demos[name]['backend_port'],
            '%s/config.toml' % backend_dir(name)])
        if returncode != 0:
            return 'stderr: %s, stdout: %s' % (stderr, stdout)
        (returncode, stdout, stderr) = run_command(['cp', '-rf', './search/variables', '%s/search/' % backend_dir(name)])
        if returncode != 0:
            return 'stderr: %s, stdout: %s' % (stderr, stdout)
        (returncode, stdout, stderr) = run_command(['cp', '-rf', './search/grammars', '%s/search/' % backend_dir(name)])
        if returncode != 0:
            return 'stderr: %s, stdout: %s' % (stderr, stdout)
        (returncode, stdout, stderr) = run_command(['cp', '-rf', './search/data', '%s/search/' % backend_dir(name)])
        if returncode != 0:
            return 'stderr: %s, stdout: %s' % (stderr, stdout)
        (returncode, stdout, stderr) = run_command(['cp', '-rf', './es/synonyms', '%s/es/' % backend_dir(name)])
        if returncode != 0:
            return 'stderr: %s, stdout: %s' % (stderr, stdout)
        if demos[name]['elastic'] == 'reindex':
            (returncode, stdout, stderr) = run_command([
                'sed', '-i', '-E',
                's/#index-date.*/index-date = \"%s\"/g' % name,
                '%s/config.toml' % backend_dir(name)])
            if returncode != 0:
                return 'stderr: %s, stdout: %s' % (stderr, stdout)
            (returncode, stdout, stderr) = run_command([
                'sed', '-i', '-E',
                's/#grammar-index-date.*/grammar-index-date = \"%s\"/g' % name,
                '%s/config.toml' % backend_dir(name)])
            if returncode != 0:
                return 'stderr: %s, stdout: %s' % (stderr, stdout)

        if original_branch != branch:
            (returncode, stdout, stderr) = run_command(['git', 'checkout', original_branch])
            if returncode != 0:
                return 'stderr: %s, stdout: %s' % (stderr, stdout)
        start_backend(name)
        return ''
    

def set_up_demo(name):
    # Start backend
    print 'Setting up demo: %s.' % demos[name]
    demos[name]['status'].append('Setting up backend')
    error = set_up_backend(name)
    if error:
        demos[name]['status'].append(error)
        return
    else:
        demos[name]['status'].append('Backend set up.')

    # Start frontend
    demos[name]['status'].append('Setting up frontend...')
    error = set_up_frontend(name, demos[name]['frontend_branch'])
    if error:
        demos[name]['status'].append(error)
        return

    # Reindex index, grammars, and update synonyms.
    if demos[name]['elastic'] == 'reindex':
        demos[name]['status'].append('Cleaning existing indexes for [%s]' % name)
        (returncode, stdout, stderr) = delete_indexes(name)
        if returncode != 0:
            print 'Failed clening existing indexes: %d %s %s' % (returncode, stdout, stderr)
            demos[name]['status'].append('Failed clening existing indexes: %d' % returncode)
        demos[name]['status'].append('Reindexing ... will take ~20 minutes.')
        error = start_reindex_grammars(name)
        if error:
            demos[name]['status'].append(error)
            return
        demos[name]['status'].append('Grammars indexed successfully.')
        error = start_reindex(name)
        if error:
            demos[name]['status'].append(error)
            return
        demos[name]['status'].append('Indexed everything successfully.')
        error = start_update_synonyms(name)
        if error:
            demos[name]['status'].append(error)
            return
        demos[name]['status'].append('Synonyms updated successfully.')

    demos[name]['status'].append('Done!')

start_queue = multiprocessing.JoinableQueue()
def queue_worker():
    while True:
        name = start_queue.get()
        set_up_demo(name)
        start_queue.task_done()

for i in range(NUM_WORKER_THREADS):
     t = threading.Thread(target=queue_worker)
     t.daemon = True
     t.start()

# Monitor calls
nextCallId = 0
calls = {}
class MonitorCalls:
    def __init__(self, message):
        global nextCallId
        global calls
        self.callId = nextCallId
        nextCallId += 1
        calls[self.callId] = message
    def __enter__(self):
        self.printCalls('Before')
        return self.callId
    def __exit__(self, type, value, traceback):
        global calls
        del calls[self.callId]
        self.printCalls('After')
    def printCalls(self, prefix):
        global calls
        print '\n%s - %d Calls:' % (prefix, len(calls))
        for (k, v) in calls.iteritems():
            print '%s - %s' % (k, v)
        print
        sys.stdout.flush()

class DemoHandler(SimpleHTTPServer.SimpleHTTPRequestHandler):
    def return_response(self, code, message):
        # print 'returning [%d]: [%s]' % (code, message)
        self.send_response(code)
        self.end_headers()
        self.wfile.write(message)

    def do_GET(self):
        with MonitorCalls(self.path):
            parts = urlparse.urlparse(self.path)
            # print 'get %s' % (parts,)
            if parts.path == '/':
                self.path = './misc/demo.html'
                return SimpleHTTPServer.SimpleHTTPRequestHandler.do_GET(self)
            if parts.path == '/status':
                self.return_response(200, json.dumps(demos_to_json()))
                return

            # Serve log files.
            m = re.match(r'^/logs/(%s)/(%s)$' % (NAME_RE, LOG_RE), parts.path)
            if m:
                filename = m.groups(1)[1]
                dirname = backend_dir(m.groups(1)[0])
                if filename == 'frontend.log': #or filename == 'ssr_frontend.log':
                    dirname = frontend_dir(m.groups(1)[0])
                path = '%s/%s' % (dirname, filename)
                text = 'Unable to read file'
                with open(path, 'r') as f:
                    text = f.read()
                self.return_response(200, text)
                return

            m = re.match(r'^/stop_and_clean/(%s)$' % NAME_RE, parts.path)
            if m:
                stop_and_clean(m.groups(1)[0])

            m = re.match(r'^/update_reload/(%s)$' % NAME_RE, parts.path)
            if m:
                update_reload(m.groups(1)[0])

            # Cannot server config.toml as it has passwords insdie...
            # m = re.match(r'^/logs/(%s)/config.toml$' % NAME_RE, parts.path)
            # if m:
            #     path = '%s/config.toml' % backend_dir(m.groups(1)[0])
            #     logs = 'Unable to read config file'
            #     with open(path, 'r') as log_file:
            #         logs = log_file.read()
            #     self.return_response(200, logs)
            #     return

    def do_POST(self):
        with MonitorCalls(self.path):
            if self.path == '/start':
                content_length = int(self.headers['Content-Length'])
                body = self.rfile.read(content_length)
                request = json.loads(body)
                print request
                print type(request)
                fields = ['name', 'comment', 'backend_branch', 'frontend_branch', 'elastic']
                missing_fields = [f for f in fields if not request[f]]
                if len(missing_fields):
                    self.return_response(400, 'Please set field values: %s.' % ', '.join(missing_fields))
                    return

                if not re.match(r'^%s$' % NAME_RE, request['name']):
                    self.return_response(400, '"name" should be simple letters, digits or underscore without spaces.')
                    return

                if request['name'] in demos:
                    self.return_response(400, 'Demo with name: [%s] already exist.' % request['name'])
                    return

                request['status'] = []
                demos[request['name']] = request
                start_queue.put(request['name'])

                self.send_response(200)

class ThreadingTCPServer(SocketServer.ThreadingMixIn, SocketServer.TCPServer):
    pass

SocketServer.TCPServer.allow_reuse_address = True
httpd = ThreadingTCPServer(("", PORT), DemoHandler)

print "serving at port", PORT
httpd.serve_forever()
start_queue.join()
