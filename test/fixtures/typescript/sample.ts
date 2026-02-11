function hello(name: string): void {
    console.log("Hello, " + name);
}
class Greeter {
    greet() { return "Hi"; }
}

function main() {
    hello("world");
    const g = new Greeter();
    g.greet();
}
