#!/usr/bin/env python3
"""
Socket.IO Load Testing Script for ChaosLabs
Tests horizontal scaling with multiple concurrent connections and message broadcasting.
"""

import asyncio
import socketio
import time
import json
import statistics
import argparse
from datetime import datetime
import threading


class SocketIOLoadTester:
    def __init__(
        self, base_url, num_clients=100, message_interval=1.0, test_duration=60
    ):
        self.base_url = base_url.rstrip("/")
        self.num_clients = num_clients
        self.message_interval = message_interval
        self.test_duration = test_duration

        self.clients = []
        self.results = {
            "connections": [],
            "messages_sent": 0,
            "messages_received": 0,
            "errors": 0,
            "latencies": [],
            "start_time": None,
            "end_time": None,
        }

        self.running = False
        self.lock = threading.Lock()

    async def create_client(self, client_id):
        """Create a Socket.IO client with event handlers"""
        sio = socketio.AsyncClient()

        @sio.event
        async def connect():
            with self.lock:
                self.results["connections"].append(
                    {
                        "client_id": client_id,
                        "connected_at": time.time(),
                        "status": "connected",
                    }
                )
            print(f"Client {client_id} connected")

        @sio.event
        async def disconnect():
            with self.lock:
                for conn in self.results["connections"]:
                    if conn["client_id"] == client_id:
                        conn["status"] = "disconnected"
                        conn["disconnected_at"] = time.time()
            print(f"Client {client_id} disconnected")

        @sio.event
        async def message(data):
            with self.lock:
                self.results["messages_received"] += 1
            print(f"Client {client_id} received message: {data}")

        @sio.event
        async def experiment_update(data):
            with self.lock:
                self.results["messages_received"] += 1
            print(f"Client {client_id} received experiment update: {data}")

        @sio.event
        async def welcome(data):
            with self.lock:
                self.results["messages_received"] += 1
            print(f"Client {client_id} received welcome: {data}")

        try:
            await sio.connect(self.base_url)
            return sio
        except Exception as e:
            with self.lock:
                self.results["errors"] += 1
            print(f"Failed to connect client {client_id}: {e}")
            return None

    async def run_client(self, client_id):
        """Run a single client for the test duration"""
        sio = await self.create_client(client_id)
        if not sio:
            return

        self.clients.append(sio)

        try:
            # Join experiment room
            room_data = {"experiment_id": f"test_exp_{client_id}"}
            await sio.emit("join_experiment_room", room_data)

            # Send periodic messages
            start_time = time.time()
            while self.running and (time.time() - start_time) < self.test_duration:
                try:
                    message = {
                        "type": "ping",
                        "client_id": client_id,
                        "timestamp": time.time(),
                        "data": f"Test message from client {client_id}",
                    }

                    send_time = time.time()
                    await sio.emit("message", message)

                    with self.lock:
                        self.results["messages_sent"] += 1
                        self.results["latencies"].append(time.time() - send_time)

                except Exception as e:
                    with self.lock:
                        self.results["errors"] += 1
                    print(f"Error sending message from client {client_id}: {e}")

                await asyncio.sleep(self.message_interval)

        except Exception as e:
            with self.lock:
                self.results["errors"] += 1
            print(f"Error in client {client_id}: {e}")
        finally:
            try:
                await sio.disconnect()
            except Exception:
                pass

    async def run_load_test(self):
        """Run the complete load test"""
        print(f"Starting Socket.IO load test:")
        print(f"  Base URL: {self.base_url}")
        print(f"  Number of clients: {self.num_clients}")
        print(f"  Message interval: {self.message_interval}s")
        print(f"  Test duration: {self.test_duration}s")
        print()

        self.results["start_time"] = time.time()
        self.running = True

        # Create and run clients
        tasks = []
        for i in range(self.num_clients):
            task = asyncio.create_task(self.run_client(f"client_{i:04d}"))
            tasks.append(task)

        # Wait for all clients to complete
        await asyncio.gather(*tasks, return_exceptions=True)

        self.running = False
        self.results["end_time"] = time.time()

        # Disconnect all clients
        for client in self.clients:
            try:
                await client.disconnect()
            except Exception:
                pass

        self.clients.clear()

    def print_results(self):
        """Print test results and statistics"""
        duration = self.results["end_time"] - self.results["start_time"]
        connected_clients = len(
            [c for c in self.results["connections"] if c["status"] == "connected"]
        )

        print("\n" + "=" * 60)
        print("SOCKET.IO LOAD TEST RESULTS")
        print("=" * 60)
        print(f"Test Duration: {duration:.2f} seconds")
        print(f"Total Clients: {self.num_clients}")
        print(f"Connected Clients: {connected_clients}")
        success_rate = (connected_clients / self.num_clients) * 100
        print(f"Connection Success Rate: {success_rate:.1f}%")
        print()

        print(f"Messages Sent: {self.results['messages_sent']}")
        print(f"Messages Received: {self.results['messages_received']}")
        print(f"Errors: {self.results['errors']}")
        print()

        if self.results["latencies"]:
            latencies = self.results["latencies"]
            print("Latency Statistics (seconds):")
            print(f"  Min: {min(latencies):.4f}")
            print(f"  Max: {max(latencies):.4f}")
            print(f"  Mean: {statistics.mean(latencies):.4f}")
            print(f"  Median: {statistics.median(latencies):.4f}")
            p95_idx = min(18, len(statistics.quantiles(latencies, n=20)) - 1)
            p99_idx = min(98, len(statistics.quantiles(latencies, n=100)) - 1)
            print(f"  P95: {statistics.quantiles(latencies, n=20)[p95_idx]:.4f}")
            print(f"  P99: {statistics.quantiles(latencies, n=100)[p99_idx]:.4f}")
            print()

        # Calculate throughput
        messages_per_second = (
            self.results["messages_sent"] / duration if duration > 0 else 0
        )
        print(f"Throughput: {messages_per_second:.2f} messages/second")

        # Connection timing
        if self.results["connections"]:
            connection_times = []
            for conn in self.results["connections"]:
                if "connected_at" in conn and "disconnected_at" in conn:
                    duration = conn["disconnected_at"] - conn["connected_at"]
                    connection_times.append(duration)

            if connection_times:
                avg_connection_time = statistics.mean(connection_times)
                print(f"Average Connection Duration: {avg_connection_time:.2f} seconds")

        print("=" * 60)

    def save_results(self, filename=None):
        """Save test results to JSON file"""
        if not filename:
            timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
            filename = f"socketio_load_test_{timestamp}.json"

        # Convert datetime objects to strings for JSON serialization
        results_copy = self.results.copy()
        results_copy["start_time"] = datetime.fromtimestamp(
            results_copy["start_time"]
        ).isoformat()
        results_copy["end_time"] = datetime.fromtimestamp(
            results_copy["end_time"]
        ).isoformat()

        for conn in results_copy["connections"]:
            if "connected_at" in conn:
                conn["connected_at"] = datetime.fromtimestamp(
                    conn["connected_at"]
                ).isoformat()
            if "disconnected_at" in conn:
                conn["disconnected_at"] = datetime.fromtimestamp(
                    conn["disconnected_at"]
                ).isoformat()

        with open(filename, "w") as f:
            json.dump(results_copy, f, indent=2)

        print(f"Results saved to: {filename}")


async def main():
    parser = argparse.ArgumentParser(description="Socket.IO Load Testing for ChaosLabs")
    parser.add_argument(
        "--url",
        default="http://localhost:5000",
        help="Base URL of the Socket.IO server",
    )
    parser.add_argument(
        "--clients", type=int, default=100, help="Number of concurrent clients"
    )
    parser.add_argument(
        "--interval", type=float, default=1.0, help="Message interval in seconds"
    )
    parser.add_argument(
        "--duration", type=int, default=60, help="Test duration in seconds"
    )
    parser.add_argument("--save", help="Save results to specified file")

    args = parser.parse_args()

    # Create and run load tester
    tester = SocketIOLoadTester(
        base_url=args.url,
        num_clients=args.clients,
        message_interval=args.interval,
        test_duration=args.duration,
    )

    try:
        await tester.run_load_test()
        tester.print_results()

        if args.save:
            tester.save_results(args.save)
        else:
            tester.save_results()

    except KeyboardInterrupt:
        print("\nTest interrupted by user")
        tester.running = False
    except Exception as e:
        print(f"Test failed: {e}")
        import traceback

        traceback.print_exc()


if __name__ == "__main__":
    asyncio.run(main())
