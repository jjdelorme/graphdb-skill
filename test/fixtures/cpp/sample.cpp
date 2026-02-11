#include <iostream>

void hello() {
    std::cout << "Hello";
}

class Greeter {
public:
    void greet() {
        hello();
    }
};
