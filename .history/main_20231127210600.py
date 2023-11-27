# Install required packages:
# pip install kubernetes websockets

from kubernetes import client, config, watch
import json
import asyncio
import websockets

# Async generator to watch Kubernetes events
async def watch_kubernetes_events():
    config.load_kube_config()
    v1 = client.CoreV1Api()
    w = watch.Watch()
    async for event in w.stream(v1.list_event_for_all_namespaces):
        yield event

# Function to format event data into human-friendly JSON
def format_event(event):
    formatted_event = {
        'type': event['type'],
        'object': {
            'kind': event['object'].kind,
            'name': event['object'].metadata.name,
            'namespace': event['object'].metadata.namespace,
            'message': event['object'].message,
            'time': event['object'].last_timestamp.strftime("%Y-%m-%d %H:%M:%S") if event['object'].last_timestamp else "N/A"
        }
    }
    return json.dumps(formatted_event, indent=4)

# Async function for WebSocket server
async def events_websocket(websocket, path):
    async for event in watch_kubernetes_events():
        formatted_event = format_event(event)
        await websocket.send(formatted_event)

# Start WebSocket server
async def main():
    server = await websockets.serve(events_websocket, "0.0.0.0", 7008)
    await server.wait_closed()

# Run the server
asyncio.run(main())
